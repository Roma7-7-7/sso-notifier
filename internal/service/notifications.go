package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

//go:generate mockgen -package mocks -destination mocks/telegram.go . TelegramClient

//go:generate mockgen -package mocks -destination mocks/notifications.go . NotificationsStore

var ErrShutdownsNotAvailable = errors.New("shutdowns not available")

type (
	TelegramClient interface {
		SendMessage(context.Context, string, string) error
	}

	NotificationsReaderStore interface {
		GetNotificationState(chatID int64, date dal.Date) (dal.NotificationState, bool, error)
	}

	NotificationsStore interface {
		NotificationsReaderStore
		PutNotificationState(state dal.NotificationState) error
		CleanupNotificationStates(olderThan time.Duration) error
	}

	Notifications struct {
		shutdowns     ShutdownsReaderStore
		subscriptions SubscriptionsReaderStore
		notifications NotificationsStore
		telegram      TelegramClient
		clock         Clock

		notificationsTTL time.Duration
		log              *slog.Logger
		mx               *sync.Mutex
	}
)

func NewNotifications(
	shutdowns ShutdownsReaderStore,
	subscriptions SubscriptionsReaderStore,
	notifications NotificationsStore,
	telegram TelegramClient,
	clock Clock,
	notificationsTTL time.Duration,
	log *slog.Logger,
) *Notifications {
	return &Notifications{
		notificationsTTL: notificationsTTL,
		shutdowns:        shutdowns,
		subscriptions:    subscriptions,
		notifications:    notifications,
		telegram:         telegram,
		clock:            clock,

		log: log.With("component", "service").With("service", "notifications"),
		mx:  &sync.Mutex{},
	}
}

func (s *Notifications) NotifyPowerSupplyScheduleUpdates(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "Notifying about shoutdown updates")

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("get all subscriptions: %w", err)
	}

	now := s.clock.Now()
	today := dal.DateByTime(now)
	tomorrow := dal.DateByTime(now.AddDate(0, 0, 1))

	for _, sub := range subs {
		s.processSubscriptionNotification(ctx, sub, today, tomorrow)
	}

	return nil
}

func (s *Notifications) NotifyPowerSupplySchedule(ctx context.Context, chatID int64) error {
	log := s.log.With("chatID", chatID)
	log.DebugContext(ctx, "Notifying about shoutdown updates")
	sub, ok, err := s.subscriptions.GetSubscription(chatID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: chatID=%d", ErrSubscriptionNotFound, chatID)
	}

	now := s.clock.Now()
	today := dal.DateByTime(now)
	tomorrow := dal.DateByTime(now.AddDate(0, 0, 1))
	text := ""
	mb, err := s.prepareMessageBuilder(ctx, sub, today, tomorrow)
	if err != nil {
		if !errors.Is(err, ErrShutdownsNotAvailable) {
			return fmt.Errorf("prepare message: %w", err)
		}
		log.DebugContext(ctx, "Shutdowns are not yet available", "error", err)
		text = "Графік стабілізаційних відключень ще не доступний. Спробуйте пізніше."
	}

	if text == "" {
		// Passing empty states because we are not going to update them in any case
		msg, err := mb.Build(sub, dal.NotificationState{}, dal.NotificationState{})
		if err != nil {
			return fmt.Errorf("build message: %w", err)
		}
		text = msg.Text
	}

	err = s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), text)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}

func (s *Notifications) Cleanup(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.log.InfoContext(ctx, "cleaning up")
	return s.notifications.CleanupNotificationStates(s.notificationsTTL) //nolint:wrapcheck // it's ok
}

func (s *Notifications) prepareMessageBuilder(ctx context.Context, sub dal.Subscription, today dal.Date, tomorrow dal.Date) (*MessageBuilder, error) {
	todayTable, ok, err := s.shutdowns.GetShutdowns(today)
	if err != nil {
		return nil, fmt.Errorf("get shutdowns table for today: %w", err)
	}
	if !ok {
		return nil, ErrShutdownsNotAvailable
	}
	var mb *MessageBuilder
	switch sub.Settings[dal.SettingShutdownsMessageFormat] {
	case nil, "", dal.ShutdownsMessageFormatLinear:
		mb = NewLinearMessageBuilder(todayTable, false, s.clock.Now())
	case dal.ShutdownsMessageFormatLinearWithRange:
		mb = NewLinearMessageBuilder(todayTable, true, s.clock.Now())
	case dal.ShutdownsMessageFormatGrouped:
		mb = NewGroupedMessageBuilder(todayTable, s.clock.Now())
	default:
		s.log.WarnContext(ctx, "Unknown shutdown message format. Fallback to default linear without range", "format", sub.Settings[dal.SettingShutdownsMessageFormat])
		mb = NewLinearMessageBuilder(todayTable, false, s.clock.Now())
	}

	tomorrowTable, hasTomorrow, err := s.shutdowns.GetShutdowns(tomorrow)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get tomorrow's shutdowns", "error", err)
	} else if hasTomorrow {
		mb.WithNextDay(tomorrowTable)
		s.log.DebugContext(ctx, "Including tomorrow's schedule in notifications")
	}

	return mb, nil
}

func (s *Notifications) processSubscriptionNotification(
	ctx context.Context,
	sub dal.Subscription,
	today, tomorrow dal.Date,
) {
	chatID := sub.ChatID
	log := s.log.With("chatID", chatID)

	msgBuilder, err := s.prepareMessageBuilder(ctx, sub, today, tomorrow)
	if err != nil {
		if errors.Is(err, ErrShutdownsNotAvailable) {
			s.log.InfoContext(ctx, "No shoutdown updates available")
			return
		}

		s.log.ErrorContext(ctx, "Failed to prepare shoutdown updates message builder", "error", err)
		return
	}

	todayState, err := s.getOrCreateNotificationState(chatID, today)
	if err != nil {
		log.ErrorContext(ctx, "failed to get or create notification state for today", "error", err)
		return
	}
	tomorrowState, err := s.getOrCreateNotificationState(chatID, tomorrow)
	if err != nil {
		log.ErrorContext(ctx, "failed to get or create notification state for tomorrow", "error", err)
		return
	}

	msg, err := msgBuilder.Build(sub, todayState, tomorrowState)
	if err != nil {
		log.ErrorContext(ctx, "failed to build message", "error", err)
		return
	}

	if len(msg.TodayUpdatedGroups) == 0 && len(msg.TomorrowUpdatedGroups) == 0 {
		return
	}

	if err := s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), msg.Text); err != nil {
		log.ErrorContext(ctx, "failed to send message", "chatID", chatID, "error", err)
		return
	}

	s.updateNotificationStates(ctx, todayState, tomorrowState, msg, log)
}

// getOrCreateNotificationState retrieves or creates notification state for a date
func (s *Notifications) getOrCreateNotificationState(chatID int64, date dal.Date) (dal.NotificationState, error) {
	state, exists, err := s.notifications.GetNotificationState(chatID, date)
	if err != nil {
		return state, fmt.Errorf("get notification state: %w", err)
	}
	if !exists {
		state = dal.NotificationState{
			ChatID: chatID,
			Date:   date.ToKey(),
			Hashes: make(map[string]string),
		}
	}
	return state, nil
}

func (s *Notifications) updateNotificationStates(
	ctx context.Context,
	todayState, tomorrowState dal.NotificationState,
	msg PowerSupplyScheduleMessage,
	log *slog.Logger,
) {
	now := s.clock.Now()

	if len(msg.TodayUpdatedGroups) > 0 {
		for groupNum, newHash := range msg.TodayUpdatedGroups {
			todayState.Hashes[groupNum] = newHash
		}
		todayState.SentAt = now

		if err := s.notifications.PutNotificationState(todayState); err != nil {
			log.ErrorContext(ctx, "failed to update today's notification state", "error", err)
		}
	}

	if len(msg.TomorrowUpdatedGroups) > 0 {
		for groupNum, newHash := range msg.TomorrowUpdatedGroups {
			tomorrowState.Hashes[groupNum] = newHash
		}
		tomorrowState.SentAt = now

		if err := s.notifications.PutNotificationState(tomorrowState); err != nil {
			log.ErrorContext(ctx, "failed to update tomorrow's notification state", "error", err)
		}
	}
}
