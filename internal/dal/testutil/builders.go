//nolint:mnd // this is testutil
package testutil

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

const (
	AllStatesOnHash  = "YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY"
	AllStatesOffHash = "NNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNNN"
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

func (b *SubscriptionBuilder) BuildMatcher(t *testing.T) *SubscriptionMatcher {
	return NewSubscriptionMatcher(t, b.sub)
}

// ShutdownsBuilder provides fluent API for building test shutdowns
type ShutdownsBuilder struct {
	shutdowns dal.Shutdowns
}

// NewShutdowns creates a new shutdowns builder with defaults
func NewShutdowns() *ShutdownsBuilder {
	return &ShutdownsBuilder{
		shutdowns: dal.Shutdowns{
			Date: "23.11.2025",
			Periods: []dal.Period{
				{From: "00:00", To: "00:30"},
				{From: "00:30", To: "01:00"},
				{From: "01:00", To: "01:30"},
				{From: "01:30", To: "02:00"},
				{From: "02:00", To: "02:30"},
				{From: "02:30", To: "03:00"},
				{From: "03:00", To: "03:30"},
				{From: "03:30", To: "04:00"},
				{From: "04:00", To: "04:30"},
				{From: "04:30", To: "05:00"},
				{From: "05:00", To: "05:30"},
				{From: "05:30", To: "06:00"},
				{From: "06:00", To: "06:30"},
				{From: "06:30", To: "07:00"},
				{From: "07:00", To: "07:30"},
				{From: "07:30", To: "08:00"},
				{From: "08:00", To: "08:30"},
				{From: "08:30", To: "09:00"},
				{From: "09:00", To: "09:30"},
				{From: "09:30", To: "10:00"},
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
				{From: "11:00", To: "11:30"},
				{From: "11:30", To: "12:00"},
				{From: "12:00", To: "12:30"},
				{From: "12:30", To: "13:00"},
				{From: "13:00", To: "13:30"},
				{From: "13:30", To: "14:00"},
				{From: "14:00", To: "14:30"},
				{From: "14:30", To: "15:00"},
				{From: "15:00", To: "15:30"},
				{From: "15:30", To: "16:00"},
				{From: "16:00", To: "16:30"},
				{From: "16:30", To: "17:00"},
				{From: "17:00", To: "17:30"},
				{From: "17:30", To: "18:00"},
				{From: "18:00", To: "18:30"},
				{From: "18:30", To: "19:00"},
				{From: "19:00", To: "19:30"},
				{From: "19:30", To: "20:00"},
				{From: "20:00", To: "20:30"},
				{From: "20:30", To: "21:00"},
				{From: "21:00", To: "21:30"},
				{From: "21:30", To: "22:00"},
				{From: "22:00", To: "22:30"},
				{From: "22:30", To: "23:00"},
				{From: "23:00", To: "23:30"},
				{From: "23:30", To: "24:00"},
			},
			Groups: map[string]dal.ShutdownGroup{
				"1":  {Number: 1, Items: ParseGroupHash(AllStatesOnHash)},
				"2":  {Number: 2, Items: ParseGroupHash(AllStatesOnHash)},
				"3":  {Number: 3, Items: ParseGroupHash(AllStatesOnHash)},
				"4":  {Number: 4, Items: ParseGroupHash(AllStatesOnHash)},
				"5":  {Number: 5, Items: ParseGroupHash(AllStatesOnHash)},
				"6":  {Number: 6, Items: ParseGroupHash(AllStatesOnHash)},
				"7":  {Number: 7, Items: ParseGroupHash(AllStatesOnHash)},
				"8":  {Number: 8, Items: ParseGroupHash(AllStatesOnHash)},
				"9":  {Number: 9, Items: ParseGroupHash(AllStatesOnHash)},
				"10": {Number: 10, Items: ParseGroupHash(AllStatesOnHash)},
				"11": {Number: 11, Items: ParseGroupHash(AllStatesOnHash)},
				"12": {Number: 12, Items: ParseGroupHash(AllStatesOnHash)},
			},
		},
	}
}

//nolint:gochecknoglobals // this is testutil
var StubGroupHashes = map[int]string{
	1:  "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY",
	2:  "MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY",
	3:  "NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN",
	4:  "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN",
	5:  "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY",
	6:  "MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY",
	7:  "NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN",
	8:  "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN",
	9:  "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY",
	10: "MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY",
	11: "NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN",
	12: "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN",
}

func (b *ShutdownsBuilder) WithStubGroups() *ShutdownsBuilder {
	for num, hash := range StubGroupHashes {
		b.shutdowns.Groups[strconv.Itoa(num)] = dal.ShutdownGroup{
			Number: num,
			Items:  ParseGroupHash(hash),
		}
	}

	return b
}

func ParseGroupHash(s string) []dal.Status {
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
func (b *ShutdownsBuilder) WithGroup(groupNum int, statuses string) *ShutdownsBuilder {
	b.shutdowns.Groups[strconv.Itoa(groupNum)] = dal.ShutdownGroup{
		Number: groupNum,
		Items:  ParseGroupHash(statuses),
	}
	return b
}

// Build returns the constructed shutdowns
func (b *ShutdownsBuilder) Build() dal.Shutdowns {
	return b.shutdowns
}

// BuildPointer returns pointer to the constructed shutdowns
func (b *ShutdownsBuilder) BuildPointer() *dal.Shutdowns {
	res := b.shutdowns
	return &res
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
