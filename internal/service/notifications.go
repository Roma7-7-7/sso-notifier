package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

//go:generate mockgen -package mocks -destination mocks/telegram.go . TelegramClient

//go:generate mockgen -package mocks -destination mocks/notifications.go . NotificationsStore

var ErrShutdownsNotAvailable = errors.New("shutdowns not available")

type (
	TelegramClient interface {
		SendMessage(context.Context, string, string) error
	}

	NotificationsStore interface {
		GetNotificationState(chatID int64, date dal.Date) (dal.NotificationState, bool, error)
		PutNotificationState(state dal.NotificationState) error
		CleanupNotificationStates(olderThan time.Duration) error
	}

	Notifications struct {
		shutdowns     ShutdownsStore
		subscriptions SubscriptionsStore
		notifications NotificationsStore
		telegram      TelegramClient
		clock         Clock

		notificationsTTL time.Duration
		log              *slog.Logger
		mx               *sync.Mutex
	}
)

func NewNotifications(
	shutdowns ShutdownsStore,
	subscriptions SubscriptionsStore,
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

func (s *Notifications) NotifyShutdownUpdates(ctx context.Context) error {
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

func (s *Notifications) Cleanup(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	s.log.InfoContext(ctx, "cleaning up")
	return s.notifications.CleanupNotificationStates(s.notificationsTTL) //nolint:wrapcheck // it's ok
}

func (s *Notifications) prepareMessageBuilder(ctx context.Context, sub dal.Subscription, today dal.Date, tomorrow dal.Date) (*messageBuilder, error) {
	todayTable, ok, err := s.shutdowns.GetShutdowns(today)
	if err != nil {
		return nil, fmt.Errorf("get shutdowns table for today: %w", err)
	}
	if !ok {
		return nil, ErrShutdownsNotAvailable
	}
	var strategy messageBuildStrategy
	switch sub.Settings[dal.SettingShutdownsMessageFormat] {
	case dal.ShutdownsMessageFormatGrouped:
		strategy = NewGroupedMessageBuilder(todayTable, s.clock.Now())
	case dal.ShutdownsMessageFormatLinear:
		strategy = NewLinearMessageBuilder(todayTable, false, s.clock.Now())
	case dal.ShutdownsMessageFormatLinearWithRange:
		strategy = NewLinearMessageBuilder(todayTable, true, s.clock.Now())
	default:
		s.log.WarnContext(ctx, "Unknown shutdown message format. Fallback to default linear without range", "format", dal.SettingShutdownsMessageFormat)
		strategy = NewLinearMessageBuilder(todayTable, false, s.clock.Now())
	}

	tomorrowTable, hasTomorrow, err := s.shutdowns.GetShutdowns(tomorrow)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get tomorrow's shutdowns", "error", err)
	} else if hasTomorrow {
		strategy.WithNextDay(tomorrowTable)
		s.log.DebugContext(ctx, "Including tomorrow's schedule in notifications")
	}

	return &messageBuilder{
		messageBuildStrategy: strategy,
	}, nil
}

// processSubscriptionNotification processes notification for a single subscription
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
		if !errors.Is(err, telegram.ErrForbidden) {
			log.ErrorContext(ctx, "failed to send message", "error", err)
			return
		}

		log.InfoContext(ctx, "bot is blocked by user. purging subscription and other data", "chatID", chatID, "error", err)
		if err := s.subscriptions.Purge(chatID); err != nil {
			log.ErrorContext(ctx, "failed to purge subscription", "chatID", chatID, "error", err)
		}
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
