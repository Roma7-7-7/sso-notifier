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

var ErrShutdownsNotAvailable = errors.New("shutdowns not available")

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
	now func() time.Time
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
		now: func() time.Time {
			return time.Now().In(loc)
		},

		log: log.With("component", "service").With("service", "notifications"),
		mx:  &sync.Mutex{},
	}
}

func (s *Notifications) NotifyShutdownUpdates(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "Notifying about shoutdown updates")

	today := dal.TodayDate(s.loc)
	msgBuilder, err := s.prepareMessageBuilder(ctx, today)
	if err != nil {
		if errors.Is(err, ErrShutdownsNotAvailable) {
			s.log.InfoContext(ctx, "No shoutdown updates available")
			return nil
		}

		return fmt.Errorf("prepare message builder: %w", err)
	}

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("getting all subscriptions: %w", err)
	}

	tomorrow := dal.TomorrowDate(s.loc)
	for _, sub := range subs {
		s.processSubscriptionNotification(ctx, sub, today, tomorrow, msgBuilder)
	}

	return nil
}

func (s *Notifications) prepareMessageBuilder(ctx context.Context, today dal.Date) (*PowerSupplyScheduleMessageBuilder, error) {
	todayTable, ok, err := s.shutdowns.GetShutdowns(today)
	if err != nil {
		return nil, fmt.Errorf("getting shutdowns table for today: %w", err)
	}
	if !ok {
		return nil, ErrShutdownsNotAvailable
	}

	msgBuilder := NewPowerSupplyScheduleMessageBuilder(todayTable, s.now())

	tomorrowTable, hasTomorrow, err := s.shutdowns.GetShutdowns(dal.TomorrowDate(s.loc))
	if err != nil {
		s.log.ErrorContext(ctx, "failed to get tomorrow's shutdowns", "error", err)
	} else if hasTomorrow {
		msgBuilder.WithNextDay(tomorrowTable)
		s.log.DebugContext(ctx, "Including tomorrow's schedule in notifications")
	}

	return msgBuilder, nil
}

// processSubscriptionNotification processes notification for a single subscription
func (s *Notifications) processSubscriptionNotification(
	ctx context.Context,
	sub dal.Subscription,
	today, tomorrow dal.Date,
	msgBuilder *PowerSupplyScheduleMessageBuilder,
) {
	chatID := sub.ChatID
	log := s.log.With("chatID", chatID)

	todayState := s.getOrCreateNotificationState(ctx, chatID, today, log)
	tomorrowState := s.getOrCreateNotificationState(ctx, chatID, tomorrow, log)

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

		s.log.InfoContext(ctx, "bot is blocked by user. purging subscription and other data", "chatID", chatID, "error", err)
		if err := s.subscriptions.Purge(chatID); err != nil {
			s.log.ErrorContext(ctx, "failed to purge subscription", "chatID", chatID, "error", err)
		}
		return
	}

	s.updateNotificationStates(ctx, todayState, tomorrowState, msg, log)
}

// getOrCreateNotificationState retrieves or creates notification state for a date
func (s *Notifications) getOrCreateNotificationState(
	ctx context.Context,
	chatID int64,
	date dal.Date,
	log *slog.Logger,
) dal.NotificationState {
	state, exists, err := s.notifications.GetNotificationState(chatID, date)
	if err != nil {
		log.ErrorContext(ctx, "failed to get notification state", "date", date.ToKey(), "error", err)
	}
	if !exists || err != nil {
		state = dal.NotificationState{
			ChatID: chatID,
			Date:   date.ToKey(),
			Hashes: make(map[string]string),
		}
	}
	return state
}

func (s *Notifications) updateNotificationStates(
	ctx context.Context,
	todayState, tomorrowState dal.NotificationState,
	msg PowerSupplyScheduleMessage,
	log *slog.Logger,
) {
	now := s.now()

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
