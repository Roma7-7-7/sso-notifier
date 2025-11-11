package testutil

import (
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
			Date:    "1 листопада",
			Periods: []dal.Period{},
			Groups:  make(map[string]dal.ShutdownGroup),
		},
	}
}

// WithDate sets the date
func (b *ShutdownsBuilder) WithDate(date string) *ShutdownsBuilder {
	b.shutdowns.Date = date
	return b
}

// WithPeriod adds a time period
func (b *ShutdownsBuilder) WithPeriod(from, to string) *ShutdownsBuilder {
	b.shutdowns.Periods = append(b.shutdowns.Periods, dal.Period{
		From: from,
		To:   to,
	})
	return b
}

// WithStandardPeriods adds standard 30-minute periods for a full day
func (b *ShutdownsBuilder) WithStandardPeriods() *ShutdownsBuilder {
	times := []string{
		"00:00", "00:30", "01:00", "01:30", "02:00", "02:30",
		"03:00", "03:30", "04:00", "04:30", "05:00", "05:30",
		"06:00", "06:30", "07:00", "07:30", "08:00", "08:30",
		"09:00", "09:30", "10:00", "10:30", "11:00", "11:30",
		"12:00", "12:30", "13:00", "13:30", "14:00", "14:30",
		"15:00", "15:30", "16:00", "16:30", "17:00", "17:30",
		"18:00", "18:30", "19:00", "19:30", "20:00", "20:30",
		"21:00", "21:30", "22:00", "22:30", "23:00", "23:30",
	}

	for i := 0; i < len(times)-1; i++ {
		b.WithPeriod(times[i], times[i+1])
	}
	b.WithPeriod("23:30", "24:00")

	return b
}

// WithGroup adds a group with status items
func (b *ShutdownsBuilder) WithGroup(groupNum string, items ...dal.Status) *ShutdownsBuilder {
	b.shutdowns.Groups[groupNum] = dal.ShutdownGroup{
		Number: parseGroupNum(groupNum),
		Items:  items,
	}
	return b
}

// Build returns the constructed shutdowns
func (b *ShutdownsBuilder) Build() dal.Shutdowns {
	return b.shutdowns
}

func parseGroupNum(s string) int {
	// Simple parser for test data
	switch s {
	case "1":
		return 1
	case "2":
		return 2
	case "3":
		return 3
	case "4":
		return 4
	case "5":
		return 5
	case "6":
		return 6
	case "7":
		return 7
	case "8":
		return 8
	case "9":
		return 9
	case "10":
		return 10
	case "11":
		return 11
	case "12":
		return 12
	default:
		return 0
	}
}

// Helper function to create multiple ON statuses
func RepeatStatus(status dal.Status, count int) []dal.Status {
	result := make([]dal.Status, count)
	for i := 0; i < count; i++ {
		result[i] = status
	}
	return result
}
