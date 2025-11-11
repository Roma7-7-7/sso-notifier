package dal

import (
	"time"
)

func (s *BoltDBTestSuite) TestBoltDB_CountSubscriptions() {
	count, err := s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(0, count)

	err = s.store.PutSubscription(NewSubscription(1).Build())
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(1, count)

	err = s.store.PutSubscription(NewSubscription(2).Build())
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(2, count)

	err = s.store.PutSubscription(NewSubscription(1).Build()) // same chat ID
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(2, count)
}

func (s *BoltDBTestSuite) TestBoltDB_ExistsSubscription() {
	ok, err := s.store.ExistsSubscription(1)
	s.Require().NoError(err, "error checking subscription")
	s.Require().False(ok)

	err = s.store.PutSubscription(NewSubscription(1).Build())
	s.Require().NoError(err, "error putting subscription")
	ok, err = s.store.ExistsSubscription(1)
	s.Require().NoError(err, "error checking subscription")
	s.Require().True(ok)
}

func (s *BoltDBTestSuite) TestBoltDB_GetSubscription() {
	startAt := time.Now()
	s.Require().NoError(s.store.PutSubscription(NewSubscription(1).Build()))

	actual, ok, err := s.store.GetSubscription(1)
	s.Require().NoError(err, "error getting subscription")
	if s.True(ok) {
		expected := NewSubscription(1).WithCreatedAt(actual.CreatedAt).Build()
		s.GreaterOrEqual(expected.CreatedAt, startAt, "subscription's CreatedAt must be after test's start at")
		s.Equal(expected, actual)
	}

	actual, ok, err = s.store.GetSubscription(2)
	s.Require().NoError(err, "error getting subscription")
	s.False(ok)
}

func (s *BoltDBTestSuite) TestBoltDB_GetAllSubscriptions() {
	s.Require().NoError(s.store.PutSubscription(NewSubscription(1).Build()))
	s.Require().NoError(s.store.PutSubscription(NewSubscription(2).Build()))
	s.Require().NoError(s.store.PutSubscription(NewSubscription(3).Build()))

	actual, err := s.store.GetAllSubscriptions()
	s.Require().NoError(err, "error getting all subscriptions")

	if s.Len(actual, 3) {
		expected := []Subscription{
			NewSubscription(1).WithCreatedAt(actual[0].CreatedAt).Build(),
			NewSubscription(2).WithCreatedAt(actual[1].CreatedAt).Build(),
			NewSubscription(3).WithCreatedAt(actual[2].CreatedAt).Build(),
		}

		s.Equal(expected, actual)
	}
}

func (s *BoltDBTestSuite) TestBoltDB_PutSubscription() {
	createdAt := time.Date(2025, time.November, 11, 18, 19, 20, 0, time.UTC).AddDate(0, 0, -2)
	s.now.Set(createdAt)

	s.Require().NoError(s.store.PutSubscription(NewSubscription(1).WithCreatedAt(time.Time{}).Build()))
	s.Require().NoError(s.store.PutSubscription(NewSubscription(2).WithCreatedAt(createdAt).Build()))
	s.Require().NoError(s.store.PutSubscription(NewSubscription(3).Build()))

	expected1 := NewSubscription(1).WithCreatedAt(createdAt).Build()
	expected2 := NewSubscription(2).WithCreatedAt(createdAt).Build()
	expected3 := NewSubscription(3).WithCreatedAt(createdAt).Build()

	s.Equal(expected1, s.mustGetSubscription(1))
	s.Equal(expected2, s.mustGetSubscription(2))
	s.Equal(expected3, s.mustGetSubscription(3))

	// make sure created at is not overridden
	s.now.Set(createdAt.Add(24 * time.Hour))
	s.Require().NoError(s.store.PutSubscription(NewSubscription(2).WithCreatedAt(createdAt.Add(24 * time.Hour)).Build()))
	s.Equal(expected2, s.mustGetSubscription(2))
}

func (s *BoltDBTestSuite) mustGetSubscription(chatID int64) Subscription {
	res, ok, err := s.store.GetSubscription(chatID)
	s.Require().NoError(err, "error getting subscription")
	s.Require().True(ok)
	return res
}

// SubscriptionBuilder provides fluent API for building test subscriptions
type SubscriptionBuilder struct {
	sub Subscription
}

// NewSubscription creates a new subscription builder with defaults
func NewSubscription(chatID int64) *SubscriptionBuilder {
	return &SubscriptionBuilder{
		sub: Subscription{
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
func (b *SubscriptionBuilder) WithSetting(key SettingKey, value interface{}) *SubscriptionBuilder {
	if b.sub.Settings == nil {
		b.sub.Settings = make(map[SettingKey]interface{})
	}
	b.sub.Settings[key] = value
	return b
}

// Build returns the constructed subscription
func (b *SubscriptionBuilder) Build() Subscription {
	return b.sub
}
