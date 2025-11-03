package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

type TelegramClient interface {
	SendMessage(context.Context, string, string) error
}

type NotificationsStore interface {
	GetNotificationState(chatID int64, date dal.Date) (dal.NotificationState, bool, error)
	PutNotificationState(state dal.NotificationState) error
}

type Notifications struct {
	shutdowns     ShutdownsStore
	subscriptions SubscriptionsStore
	notifications NotificationsStore
	telegram      TelegramClient

	loc *time.Location
	log *slog.Logger
	mx  *sync.Mutex
}

func NewNotifications(
	shutdowns ShutdownsStore,
	subscriptions SubscriptionsStore,
	notifications NotificationsStore,
	telegram TelegramClient,
	loc *time.Location,
	log *slog.Logger,
) *Notifications {
	return &Notifications{
		shutdowns:     shutdowns,
		subscriptions: subscriptions,
		notifications: notifications,
		telegram:      telegram,

		loc: loc,

		log: log.With("component", "service").With("service", "notifications"),
		mx:  &sync.Mutex{},
	}
}

func (s *Notifications) NotifyShutdownUpdates(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "Notifying about shoutdown updates")

	today := dal.TodayDate(s.loc)
	table, ok, err := s.shutdowns.GetShutdowns(today)
	if err != nil {
		return fmt.Errorf("getting shutdowns table: %w", err)
	}
	if !ok {
		// table is not ready yet
		s.log.InfoContext(ctx, "No shoutdown updates available")
		return nil
	}

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("getting all subscriptions: %w", err)
	}

	// Create reusable message builder for today's shutdowns
	now := time.Now().In(s.loc)
	msgBuilder := NewMessageBuilder(table.Date, table, now)

	for _, sub := range subs {
		chatID := sub.ChatID
		log := s.log.With("chatID", chatID)

		// Get notification state for this user and date
		notifState, exists, err := s.notifications.GetNotificationState(chatID, today)
		if err != nil {
			log.ErrorContext(ctx, "failed to get notification state", "error", err)
			continue
		}

		// If notification state doesn't exist, create an empty one
		if !exists {
			notifState = dal.NotificationState{
				ChatID: chatID,
				Date:   today.ToKey(),
				Hashes: make(map[string]string),
			}
		}

		// Build message
		msg, err := msgBuilder.Build(sub, notifState)
		if err != nil {
			log.ErrorContext(ctx, "failed to build message", "error", err)
			continue
		}

		if len(msg.UpdatedGroups) == 0 {
			continue
		}

		// Send notification
		if err := s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), msg.Text); err != nil {
			log.ErrorContext(ctx, "failed to send message", "error", err)
			continue
		}

		// Update notification state with new hashes
		for groupNum, newHash := range msg.UpdatedGroups {
			notifState.Hashes[groupNum] = newHash
		}
		notifState.SentAt = time.Now()

		if err := s.notifications.PutNotificationState(notifState); err != nil {
			log.ErrorContext(ctx, "failed to update notification state", "error", err)
			continue
		}
	}

	return nil
}
