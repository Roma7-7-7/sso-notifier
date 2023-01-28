package main

import (
	"errors"

	"go.uber.org/zap"
)

type CoreService struct {
	db     Store
	sender Sender
}

func (cs *CoreService) SendQueuedNotifications() {
	ns, err := cs.db.GetQueuedNotifications()
	if err != nil {
		zap.L().Error("failed to get queued notifications", zap.Error(err))
		return
	}
	for _, n := range ns {
		subID := zap.Int64("subscriberID", n.Target.ChatID)
		notificationID := zap.Int("notificationID", n.ID)

		if err = cs.sender.Send(n.Target, n.Msg); errors.Is(err, ErrBlockedByUser) {
			zap.L().Debug("bot is banned, removing subscriber and all related data", subID)
			if err = cs.db.PurgeSubscriber(n.Target); err != nil {
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
