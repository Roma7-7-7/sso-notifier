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

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

const (
	// defaultAlertWindowMinutes is the default number of minutes before an event to send alerts
	defaultAlertWindowMinutes = 10
)

type AlertsStore interface {
	GetAlert(key dal.AlertKey) (time.Time, bool, error)
	PutAlert(key dal.AlertKey, sentAt time.Time) error
}

type Alerts struct {
	shutdowns     ShutdownsStore
	subscriptions SubscriptionsStore
	alerts        AlertsStore
	telegram      TelegramClient

	loc *time.Location
	now func() time.Time
	log *slog.Logger
	mx  *sync.Mutex
}

// PendingAlert represents a detected outage start that needs notification
type PendingAlert struct {
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
	loc *time.Location,
	log *slog.Logger,
) *Alerts {
	return &Alerts{
		shutdowns:     shutdowns,
		subscriptions: subscriptions,
		alerts:        alerts,
		telegram:      telegram,

		loc: loc,
		now: func() time.Time {
			return time.Now().In(loc)
		},

		log: log.With("component", "service").With("service", "alerts"),
		mx:  &sync.Mutex{},
	}
}

// NotifyUpcomingShutdowns checks for upcoming status changes and sends advance notifications
func (s *Alerts) NotifyUpcomingShutdowns(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	now := s.now()
	s.log.InfoContext(ctx, "checking for upcoming shutdowns", "time", now.Format("15:04"))

	// Check if we're within notification window (6 AM - 11 PM)
	if !isWithinNotificationWindow(now.Hour()) {
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

	pendingAlerts := make([]PendingAlert, 0)

	for groupNum, group := range shutdowns.Groups {
		periodIndex, err := findPeriodIndex(shutdowns.Periods, targetTime)
		if err != nil {
			s.log.ErrorContext(ctx, "failed to find period index", "groupNum", groupNum, "targetTime", targetTime, "err", err)
			continue
		}

		for _, status := range []dal.Status{dal.OFF, dal.MAYBE, dal.ON} {
			if isOutageStart(group.Items, periodIndex, status) {
				pendingAlerts = append(pendingAlerts, PendingAlert{
					GroupNum:  groupNum,
					Date:      shutdowns.Date,
					StartTime: shutdowns.Periods[periodIndex].From,
					Status:    status,
				})
				s.log.DebugContext(ctx, "found outage start",
					"group", groupNum,
					"status", status,
					"time", shutdowns.Periods[periodIndex].From)
			}
		}
	}

	if len(pendingAlerts) == 0 {
		s.log.DebugContext(ctx, "no outage starts found")
		return nil
	}

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("get all subscriptions: %w", err)
	}

	for _, sub := range subs {
		if err := s.processSubscriptionAlert(ctx, sub, pendingAlerts, now); err != nil {
			s.log.ErrorContext(ctx, "failed to process subscription alert",
				"chatID", sub.ChatID,
				"error", err)
		}
	}

	return nil
}

// processSubscriptionAlert processes alerts for a single subscription
func (s *Alerts) processSubscriptionAlert(
	ctx context.Context,
	sub dal.Subscription,
	pendingAlerts []PendingAlert,
	now time.Time,
) error {
	chatID := sub.ChatID
	log := s.log.With("chatID", chatID)

	userAlerts := make([]PendingAlert, 0)

	for _, alert := range pendingAlerts {
		if _, subscribed := sub.Groups[alert.GroupNum]; !subscribed {
			continue
		}

		settingKey := getSettingKeyForStatus(alert.Status)
		if !dal.GetBoolSetting(sub.Settings, settingKey, false) {
			log.DebugContext(ctx, "user disabled notification",
				"group", alert.GroupNum,
				"status", alert.Status)
			continue
		}

		alertKey := dal.BuildAlertKey(chatID, alert.Date, alert.StartTime, string(alert.Status), alert.GroupNum)
		if _, exists, err := s.alerts.GetAlert(alertKey); err != nil {
			log.ErrorContext(ctx, "failed to check alert", "key", alertKey, "error", err)
			continue
		} else if exists {
			log.DebugContext(ctx, "alert already sent", "key", alertKey)
			continue
		}

		userAlerts = append(userAlerts, alert)
	}

	if len(userAlerts) == 0 {
		return nil
	}

	message := renderUpcomingMessage(userAlerts)
	if err := s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), message); err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}

	log.InfoContext(ctx, "sent upcoming notification", "alertCount", len(userAlerts))

	for _, alert := range userAlerts {
		alertKey := dal.BuildAlertKey(chatID, alert.Date, alert.StartTime, string(alert.Status), alert.GroupNum)
		if err := s.alerts.PutAlert(alertKey, now); err != nil {
			log.ErrorContext(ctx, "failed to mark alert as sent", "key", alertKey, "error", err)
			// Continue marking others
		}
	}

	return nil
}

// isOutageStart checks if the period at index i is the START of a new outage
func isOutageStart(items []dal.Status, index int, status dal.Status) bool {
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

// findPeriodIndex finds the index of the period that contains the given time
// A period contains a time if: period.From <= time < period.To
func findPeriodIndex(periods []dal.Period, targetTime time.Time) (int, error) {
	targetHour := targetTime.Hour()
	targetMin := targetTime.Minute()
	targetMinutes := targetHour*60 + targetMin

	for i, period := range periods {
		fromMinutes, err := parseTimeToMinutes(period.From)
		if err != nil {
			return 0, fmt.Errorf("parse period from: %w", err)
		}

		toMinutes, err := parseTimeToMinutes(period.To)
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

// parseTimeToMinutes parses a time string (e.g., "10:30" or "24:00") to total minutes since midnight
// "24:00" is treated as 1440 minutes (end of day)
func parseTimeToMinutes(timeStr string) (int, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour: %w", err)
	}

	min, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute: %w", err)
	}

	// Handle "24:00" as end of day (1440 minutes)
	if hour == 24 && min == 0 {
		return 24 * 60, nil
	}

	return hour*60 + min, nil
}

// isWithinNotificationWindow checks if the hour is within the notification window (6 AM - 11 PM)
func isWithinNotificationWindow(hour int) bool {
	return hour >= 6 && hour < 23
}

// getSettingKeyForStatus returns the setting key for a given status
func getSettingKeyForStatus(status dal.Status) dal.SettingKey {
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

// renderUpcomingMessage renders the notification message for upcoming events
func renderUpcomingMessage(alerts []PendingAlert) string {
	type groupKey struct {
		Status    dal.Status
		StartTime string
	}

	grouped := make(map[groupKey][]string)
	for _, a := range alerts {
		key := groupKey{Status: a.Status, StartTime: a.StartTime}
		grouped[key] = append(grouped[key], a.GroupNum)
	}

	var sb strings.Builder

	hasOn := false
	for key := range grouped {
		if key.Status == dal.ON {
			hasOn = true
			break
		}
	}

	if hasOn {
		sb.WriteString("⚡ Гарні новини! Через 10 хвилин:\n\n")
	} else {
		sb.WriteString("⚠️ Увага! Через 10 хвилин:\n\n")
	}

	for key, groups := range grouped {
		emoji := getEmojiForStatus(key.Status)
		label := getLabelForStatus(key.Status)

		if len(groups) == 1 {
			sb.WriteString(fmt.Sprintf("Група %s:\n", groups[0]))
		} else {
			sb.WriteString(fmt.Sprintf("Групи %s:\n", strings.Join(groups, ", ")))
		}

		sb.WriteString(fmt.Sprintf("%s %s об %s\n", emoji, label, key.StartTime))
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String())
}

// getEmojiForStatus returns the emoji for a status
func getEmojiForStatus(status dal.Status) string {
	switch status {
	case dal.ON:
		return "🟢"
	case dal.OFF:
		return "🔴"
	case dal.MAYBE:
		return "🟡"
	default:
		return "⚪"
	}
}

// getLabelForStatus returns the Ukrainian label for a status
func getLabelForStatus(status dal.Status) string {
	switch status {
	case dal.ON:
		return "Відновлення електроенергії"
	case dal.OFF:
		return "Відключення електроенергії"
	case dal.MAYBE:
		return "Можливе відключення електроенергії"
	default:
		return "Невідомий статус"
	}
}
