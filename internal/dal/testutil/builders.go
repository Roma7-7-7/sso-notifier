package testutil

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

// SubscriptionBuilder provides fluent API for building test subscriptions
type SubscriptionBuilder struct {
	sub dal.Subscription
}

// NewSubscription creates a new subscription builder with defaults
func NewSubscription(chatID int64) *SubscriptionBuilder {
	return &SubscriptionBuilder{
		sub: dal.Subscription{
			ChatID:    chatID,
			Groups:    make(map[string]struct{}),
			CreatedAt: time.Now(),
			Settings:  nil,
		},
	}
}

// WithGroup adds a group subscription
func (b *SubscriptionBuilder) WithGroup(groupNum string) *SubscriptionBuilder {
	b.sub.Groups[groupNum] = struct{}{}
	return b
}

// WithGroups adds multiple group subscriptions
func (b *SubscriptionBuilder) WithGroups(groupNums ...string) *SubscriptionBuilder {
	for _, g := range groupNums {
		b.sub.Groups[g] = struct{}{}
	}
	return b
}

// WithCreatedAt sets the creation time
func (b *SubscriptionBuilder) WithCreatedAt(t time.Time) *SubscriptionBuilder {
	b.sub.CreatedAt = t
	return b
}

// WithSetting sets a setting value
func (b *SubscriptionBuilder) WithSetting(key dal.SettingKey, value interface{}) *SubscriptionBuilder {
	if b.sub.Settings == nil {
		b.sub.Settings = make(map[dal.SettingKey]interface{})
	}
	b.sub.Settings[key] = value
	return b
}

// Build returns the constructed subscription
func (b *SubscriptionBuilder) Build() dal.Subscription {
	return b.sub
}

// ShutdownsBuilder provides fluent API for building test shutdowns
type ShutdownsBuilder struct {
	shutdowns dal.Shutdowns
}

// NewShutdowns creates a new shutdowns builder with defaults
func NewShutdowns() *ShutdownsBuilder {
	return &ShutdownsBuilder{
		shutdowns: dal.Shutdowns{
			Date: "2025-11-23",
			Periods: []dal.Period{
				{"00:00", "00:30"},
				{"00:30", "01:00"},
				{"01:00", "01:30"},
				{"01:30", "02:00"},
				{"02:00", "02:30"},
				{"02:30", "03:00"},
				{"03:00", "03:30"},
				{"03:30", "04:00"},
				{"04:00", "04:30"},
				{"04:30", "05:00"},
				{"05:00", "05:30"},
				{"05:30", "06:00"},
				{"06:00", "06:30"},
				{"06:30", "07:00"},
				{"07:00", "07:30"},
				{"07:30", "08:00"},
				{"08:00", "08:30"},
				{"08:30", "09:00"},
				{"09:00", "09:30"},
				{"09:30", "10:00"},
				{"10:00", "10:30"},
				{"10:30", "11:00"},
				{"11:00", "11:30"},
				{"11:30", "12:00"},
				{"12:00", "12:30"},
				{"12:30", "13:00"},
				{"13:00", "13:30"},
				{"13:30", "14:00"},
				{"14:00", "14:30"},
				{"14:30", "15:00"},
				{"15:00", "15:30"},
				{"15:30", "16:00"},
				{"16:00", "16:30"},
				{"16:30", "17:00"},
				{"17:00", "17:30"},
				{"17:30", "18:00"},
				{"18:00", "18:30"},
				{"18:30", "19:00"},
				{"19:00", "19:30"},
				{"19:30", "20:00"},
				{"20:00", "20:30"},
				{"20:30", "21:00"},
				{"21:00", "21:30"},
				{"21:30", "22:00"},
				{"22:00", "22:30"},
				{"22:30", "23:00"},
				{"23:00", "23:30"},
				{"23:30", "24:00"},
			},
			Groups: map[string]dal.ShutdownGroup{
				"1":  {1, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"2":  {2, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"3":  {3, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"4":  {4, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
				"5":  {5, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"6":  {6, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"7":  {7, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"8":  {8, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
				"9":  {9, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"10": {10, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"11": {11, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"12": {12, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
			},
		},
	}
}

func parseGroupStatuses(s string) []dal.Status {
	if len(s) != 48 {
		panic(fmt.Sprintf("expecting 48 groups, got %d", len(s)))
	}

	res := make([]dal.Status, 48)
	for i, c := range s {
		res[i] = dal.Status(c)
	}

	return res
}

// WithDate sets the date
func (b *ShutdownsBuilder) WithDate(date string) *ShutdownsBuilder {
	b.shutdowns.Date = date
	return b
}

// WithGroup adds a group with status items
func (b *ShutdownsBuilder) WithGroup(groupNum int, items ...dal.Status) *ShutdownsBuilder {
	b.shutdowns.Groups[strconv.Itoa(groupNum)] = dal.ShutdownGroup{
		Number: groupNum,
		Items:  items,
	}
	return b
}

// Build returns the constructed shutdowns
func (b *ShutdownsBuilder) Build() dal.Shutdowns {
	return b.shutdowns
}

// NotificationStateBuilder provides fluent API for building test notification states
type NotificationStateBuilder struct {
	state dal.NotificationState
}

// NewNotificationState creates a new notification state builder with defaults
func NewNotificationState(chatID int64, date dal.Date) *NotificationStateBuilder {
	return &NotificationStateBuilder{
		state: dal.NotificationState{
			ChatID: chatID,
			Date:   date.ToKey(),
			SentAt: time.Now().UTC(),
			Hashes: map[string]string{
				"1": "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY",
				"2": "MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY",
				"3": "NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN",
				"4": "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN",
			},
		},
	}
}

// WithSentAt sets the sent at time
func (b *NotificationStateBuilder) WithSentAt(t time.Time) *NotificationStateBuilder {
	b.state.SentAt = t
	return b
}

// WithHash adds a single hash for a group
func (b *NotificationStateBuilder) WithHash(group, hash string) *NotificationStateBuilder {
	b.state.Hashes[group] = hash
	return b
}

// Build returns the constructed notification state
func (b *NotificationStateBuilder) Build() dal.NotificationState {
	return b.state
}

// WithHashes sets multiple hashes at once
func (b *NotificationStateBuilder) WithHashes(hashes map[string]string) *NotificationStateBuilder {
	b.state.Hashes = hashes
	return b
}
