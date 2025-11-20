package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

const (
	// defaultAlertWindowMinutes is the default number of minutes before an event to send alerts
	defaultAlertWindowMinutes = 10
)

//go:generate mockgen -package mocks -destination mocks/alerts.go . AlertsStore

type AlertsStore interface {
	GetAlert(key dal.AlertKey) (time.Time, bool, error)
	PutAlert(key dal.AlertKey, sentAt time.Time) error
}

type (
	Alerts struct {
		shutdowns      ShutdownsStore
		subscriptions  SubscriptionsStore
		store          AlertsStore
		telegram       TelegramClient
		messageBuilder *PowerSupplyChangeMessageBuilder

		clock Clock
		loc   *time.Location
		log   *slog.Logger
		mx    *sync.Mutex
	}
)

// Alert represents a detected outage start that needs notification
type Alert struct {
	GroupNum  string
	Date      string
	StartTime string
	Status    dal.Status
}

func NewAlerts(
	shutdowns ShutdownsStore,
	subscriptions SubscriptionsStore,
	alerts AlertsStore,
	telegram TelegramClient,
	clock Clock,
	loc *time.Location,
	log *slog.Logger,
) *Alerts {
	return &Alerts{
		shutdowns:      shutdowns,
		subscriptions:  subscriptions,
		store:          alerts,
		telegram:       telegram,
		messageBuilder: NewPowerSupplyChangeMessageBuilder(),

		clock: clock,
		loc:   loc,

		log: log.With("component", "service").With("service", "alerts"),
		mx:  &sync.Mutex{},
	}
}

// NotifyPowerSupplyChanges checks for upcoming status changes and sends advance notifications
func (s *Alerts) NotifyPowerSupplyChanges(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	now := s.clock.Now()
	s.log.InfoContext(ctx, "checking for upcoming shutdowns", "time", now.Format("15:04"))

	// Check if we're within notification window (6 AM - 11 PM)
	if !IsWithinNotificationWindow(now.Hour()) {
		s.log.DebugContext(ctx, "outside notification window", "hour", now.Hour())
		return nil
	}

	targetTime := now.Add(defaultAlertWindowMinutes * time.Minute)
	s.log.DebugContext(ctx, "checking for events", "targetTime", targetTime.Format("15:04"))

	today := dal.TodayDate(s.loc)
	shutdowns, ok, err := s.shutdowns.GetShutdowns(today)
	if err != nil {
		return fmt.Errorf("get shutdowns: %w", err)
	}
	if !ok {
		s.log.DebugContext(ctx, "no schedule available")
		return nil
	}

	alerts, err := PreparePowerSupplyChangeAlerts(shutdowns, now, targetTime)
	if err != nil {
		return fmt.Errorf("prepare power supply change alerts: %w", err)
	} else if len(alerts) == 0 {
		s.log.DebugContext(ctx, "no outage starts found")
		return nil
	}

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("get all subscriptions: %w", err)
	}

	for _, sub := range subs {
		if err := s.processSubscriptionAlert(ctx, sub, alerts, now); err != nil {
			s.log.ErrorContext(ctx, "failed to process subscription alert",
				"chatID", sub.ChatID,
				"error", err)
		}
	}

	return nil
}

func (s *Alerts) processSubscriptionAlert(
	ctx context.Context,
	sub dal.Subscription,
	allAlerts []Alert,
	now time.Time,
) error {
	chatID := sub.ChatID
	log := s.log.With("chatID", chatID)

	filteredAlerts := make([]Alert, 0, len(allAlerts))

	for _, alert := range allAlerts {
		if _, subscribed := sub.Groups[alert.GroupNum]; !subscribed {
			continue
		}

		settingKey := GetSettingKeyForStatus(alert.Status)
		if !dal.GetBoolSetting(sub.Settings, settingKey, false) {
			continue
		}

		alertKey := dal.BuildAlertKey(sub.ChatID, alert.Date, alert.StartTime, string(alert.Status), alert.GroupNum)
		if _, exists, err := s.store.GetAlert(alertKey); err != nil {
			return fmt.Errorf("get alert with key %s: %w", alertKey, err)
		} else if exists {
			// alert already sent
			continue
		}

		filteredAlerts = append(filteredAlerts, alert)
	}

	message := s.messageBuilder.Build(filteredAlerts)
	if message == "" {
		return nil
	}
	if err := s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), message); err != nil {
		if !errors.Is(err, telegram.ErrForbidden) {
			return fmt.Errorf("send telegram message: %w", err)
		}

		s.log.InfoContext(ctx, "bot is blocked by user. purging subscription and other data", "chatID", chatID, "error", err)
		if err := s.subscriptions.Purge(chatID); err != nil {
			s.log.ErrorContext(ctx, "failed to purge subscription", "chatID", chatID, "error", err)
		}
		return nil
	}

	log.InfoContext(ctx, "sent upcoming notification", "alertCount", len(filteredAlerts))

	for _, alert := range filteredAlerts {
		alertKey := dal.BuildAlertKey(chatID, alert.Date, alert.StartTime, string(alert.Status), alert.GroupNum)
		if err := s.store.PutAlert(alertKey, now); err != nil {
			log.ErrorContext(ctx, "failed to mark alert as sent", "key", alertKey, "error", err)
			// Continue marking others
		}
	}

	return nil
}

func PreparePowerSupplyChangeAlerts(shutdowns dal.Shutdowns, now time.Time, target time.Time) ([]Alert, error) {
	res := make([]Alert, 0, 10) //nolint:mnd // default

	periodIndex, err := FindPeriodIndex(shutdowns.Periods, target)
	if err != nil {
		return nil, fmt.Errorf("find period index: %w", err)
	}

	period := shutdowns.Periods[periodIndex]

	// Check if the period's start time is close to our target time
	// This prevents notifying about periods that already started in the past
	periodStartMinutes, err := ParseTimeToMinutes(period.From)
	if err != nil {
		return nil, fmt.Errorf("parse period start minutes: %w", err)
	}

	nowMinutes := now.Hour()*60 + now.Minute() //nolint:mnd // hours to minutes
	periodStartAbsoluteMinutes := nowMinutes + defaultAlertWindowMinutes

	// Allow some tolerance (±5 minutes) since periods are 30-min intervals, and we check every minute
	const toleranceMinutes = 5
	if periodStartMinutes < periodStartAbsoluteMinutes-toleranceMinutes ||
		periodStartMinutes > periodStartAbsoluteMinutes+toleranceMinutes {
		return nil, nil
	}

	for groupNum, group := range shutdowns.Groups {
		for _, status := range []dal.Status{dal.OFF, dal.MAYBE, dal.ON} {
			if IsOutageStart(group.Items, periodIndex, status) {
				res = append(res, Alert{
					GroupNum:  groupNum,
					Date:      shutdowns.Date,
					StartTime: period.From,
					Status:    status,
				})
			}
		}
	}

	return res, nil
}

// IsOutageStart checks if the period at index i is the START of a new outage
func IsOutageStart(items []dal.Status, index int, status dal.Status) bool {
	if index < 0 || index >= len(items) {
		return false
	}

	currentStatus := items[index]

	if currentStatus != status {
		return false
	}

	if index == 0 {
		return true
	}

	previousStatus := items[index-1]
	return previousStatus != currentStatus
}

// FindPeriodIndex finds the index of the period that contains the given time
// A period contains a time if: period.From <= time < period.To
func FindPeriodIndex(periods []dal.Period, targetTime time.Time) (int, error) {
	targetHour := targetTime.Hour()
	targetMin := targetTime.Minute()
	targetMinutes := targetHour*60 + targetMin //nolint:mnd // hours to minutes

	for i, period := range periods {
		fromMinutes, err := ParseTimeToMinutes(period.From)
		if err != nil {
			return 0, fmt.Errorf("parse period from: %w", err)
		}

		toMinutes, err := ParseTimeToMinutes(period.To)
		if err != nil {
			return 0, fmt.Errorf("parse period to: %w", err)
		}

		// Check if targetTime falls within this period: [From, To)
		// Note: To is exclusive to avoid overlap between periods
		if targetMinutes >= fromMinutes && targetMinutes < toMinutes {
			return i, nil
		}
	}
	return 0, errors.New("no matching period")
}

// ParseTimeToMinutes parses a time string (e.g., "10:30" or "24:00") to total minutes since midnight
// "24:00" is treated as 1440 minutes (end of day)
func ParseTimeToMinutes(timeStr string) (int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 { //nolint:mnd // HH:mm
		return 0, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour: %w", err)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute: %w", err)
	}

	// Handle "24:00" as end of day (1440 minutes)
	if hour == 24 && minute == 0 {
		return 24 * 60, nil //nolint:mnd // hours to minutes
	}

	return hour*60 + minute, nil
}

// IsWithinNotificationWindow checks if the hour is within the notification window (6 AM - 11 PM)
func IsWithinNotificationWindow(hour int) bool {
	return hour >= 6 && hour < 23
}

// GetSettingKeyForStatus returns the setting key for a given status
func GetSettingKeyForStatus(status dal.Status) dal.SettingKey {
	switch status {
	case dal.OFF:
		return dal.SettingNotifyOff
	case dal.MAYBE:
		return dal.SettingNotifyMaybe
	case dal.ON:
		return dal.SettingNotifyOn
	default:
		return ""
	}
}
