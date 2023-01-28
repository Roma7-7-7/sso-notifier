package main

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
)

const subscribersLimit = 200

var ErrSubscribersLimitReached = errors.New("subscribers limit reached")

type Store interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscriptions() ([]Subscription, error)
	SetSubscription(chatID int64, groupNum string) (Subscription, error)
	NumSubscribers() (int, error)
	UpdateSubscription(sub Subscription) error
	PurgeSubscriptions(chatID int64) error

	UpdateShutdownsTable(st ShutdownsTable) error
	GetShutdownsTable() (ShutdownsTable, error)

	QueueNotification(chatID int64, msg string) (Notification, error)
	GetQueuedNotifications() ([]Notification, error)
	DeleteNotification(id int) error
}

type Sender interface {
	Send(chatID int64, msg string) error
}

type CoreService struct {
	db     Store
	sender Sender
}

func (cs *CoreService) IsSubscribed(chatID int64) (bool, error) {
	return cs.db.IsSubscribed(chatID)
}

func (cs *CoreService) GetSubscriptions() ([]Subscription, error) {
	return cs.db.GetSubscriptions()
}

func (cs *CoreService) SubscribeToGroup(chatID int64, groupNum string) (Subscription, error) {
	numSubscribers, err := cs.db.NumSubscribers()
	if err != nil {
		return Subscription{}, fmt.Errorf("failed to get number of subscribers: %w", err)
	}
	subscribed, err := cs.IsSubscribed(chatID)
	if err != nil {
		return Subscription{}, fmt.Errorf("failed to check if user is subscribed: %w", err)
	}
	if !subscribed && numSubscribers >= subscribersLimit {
		return Subscription{}, ErrSubscribersLimitReached
	}

	if !subscribed {
		zap.L().Debug("new subscriber", zap.Int64("chatID", chatID))
	}

	return cs.db.SetSubscription(chatID, groupNum)
}

func (cs *CoreService) UpdateSubscription(sub Subscription) error {
	return cs.db.UpdateSubscription(sub)
}

func (cs *CoreService) Unsubscribe(chatID int64) error {
	return cs.db.PurgeSubscriptions(chatID)
}

func (cs *CoreService) UpdateShutdownsTable(st ShutdownsTable) error {
	return cs.db.UpdateShutdownsTable(st)
}

func (cs *CoreService) GetShutdownsTable() (ShutdownsTable, bool, error) {
	table, err := cs.db.GetShutdownsTable()

	if errors.Is(err, ErrNotFound) {
		return table, false, nil
	} else if err != nil {
		return table, false, fmt.Errorf("failed to get shutdowns table: %w", err)
	}

	return table, true, nil
}

func (cs *CoreService) SendQueuedNotifications() {
	ns, err := cs.db.GetQueuedNotifications()
	if err != nil {
		zap.L().Error("failed to get queued notifications", zap.Error(err))
		return
	}
	for _, n := range ns {
		subID := zap.Int64("subscriberID", n.Target)
		notificationID := zap.Int("notificationID", n.ID)

		if err = cs.sender.Send(n.Target, n.Msg); errors.Is(err, ErrBlockedByUser) {
			zap.L().Debug("bot is banned, removing subscriber and all related data", subID)
			if err = cs.db.PurgeSubscriptions(n.Target); err != nil {
				zap.L().Error("failed to purge subscriber", zap.Error(err), subID)
			}
			continue
		} else if err != nil {
			zap.L().Error("failed to send notification", zap.Error(err), subID, notificationID)
			continue
		}
		if err = cs.db.DeleteNotification(n.ID); err != nil {
			zap.L().Error("failed to delete notification from queue", zap.Error(err), subID, notificationID)
			continue
		}
		zap.L().Debug("notification sent", subID, notificationID)
	}
}

func NewCoreService(db Store, sender Sender) *CoreService {
	return &CoreService{db: db, sender: sender}
}
