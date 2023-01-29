package main

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

const notificationInterval = 5 * time.Minute
const refreshTableInterval = 15 * time.Minute
const sendUpdatesInterval = 5 * time.Second

type Scheduler struct {
	service Service
	sender  Sender
}

func (s *Scheduler) SendNotificationsTask() {
	for {
		s.service.SendQueuedNotifications()
		time.Sleep(notificationInterval)
	}
}

func (s *Scheduler) RefreshTable() {
	for {
		page, err := loadPage()
		if err != nil {
			zap.L().Error("failed to load page", zap.Error(err))
			time.Sleep(refreshTableInterval)
			continue
		}
		table, err := parseShutdownsPage(page)
		if err != nil {
			zap.L().Error("failed to parse page", zap.Error(err))
			time.Sleep(refreshTableInterval)
			continue
		}
		if err = s.service.UpdateShutdownsTable(table); err != nil {
			zap.L().Error("failed to update shutdowns table", zap.Error(err))
			time.Sleep(refreshTableInterval)
			continue
		}

		time.Sleep(refreshTableInterval)
	}
}

func (s *Scheduler) SendUpdates() {
	for {
		table, ok, err := s.service.GetShutdownsTable()
		if err != nil {
			zap.L().Error("failed to get shutdowns table", zap.Error(err))
			time.Sleep(sendUpdatesInterval)
			continue
		}
		if !ok {
			// table is not ready yet
			time.Sleep(sendUpdatesInterval)
			continue
		}
		grouped := make(map[string]ShutdownGroup)
		for k, v := range table.Groups {
			grouped[k] = v
		}

		subs, err := s.service.GetSubscriptions()
		if err != nil {
			zap.L().Error("failed to get subscriptions", zap.Error(err))
			time.Sleep(sendUpdatesInterval)
			continue
		}

		for _, sub := range subs {
			s.processSubscription(sub, table, grouped)
		}
		time.Sleep(sendUpdatesInterval)
	}
}

func (s *Scheduler) processSubscription(sub Subscription, table ShutdownsTable, grouped map[string]ShutdownGroup) {
	msgs := make([]string, 0)

	chatID := sub.ChatID
	zapChatID := zap.Int64("chatID", chatID)
	for groupNum, hash := range sub.Groups {
		newHash := grouped[groupNum].Hash()
		if hash == newHash {
			continue
		}
		msg, err := renderGroup(groupNum, table.Periods, grouped[groupNum].Items)
		if err != nil {
			zap.L().Error("failed to render group message", zap.Error(err), zapChatID, zap.String("group", groupNum))
			return
		}
		msgs = append(msgs, msg)
		sub.Groups[groupNum] = newHash
	}

	if len(msgs) == 0 {
		return
	}

	msg, err := renderMessage(table.Date, msgs)
	if err != nil {
		zap.L().Error("failed to render message", zap.Error(err), zapChatID)
		return
	}
	if err := s.sender.Send(chatID, msg); errors.Is(err, ErrBlockedByUser) {
		zap.L().Debug("bot is banned, removing subscriber and all related data", zapChatID)
		if err = s.service.Unsubscribe(chatID); err != nil {
			zap.L().Error("failed to purge subscriber", zap.Error(err), zapChatID)
		}
		return
	} else if err != nil {
		zap.L().Error("failed to send message", zap.Error(err), zapChatID)
		return
	}

	if err := s.service.UpdateSubscription(sub); err != nil {
		zap.L().Error("failed to update subscription", zap.Error(err), zapChatID)
		return
	}
}

func NewScheduler(service Service, sender Sender) *Scheduler {
	return &Scheduler{
		service: service,
		sender:  sender,
	}
}
