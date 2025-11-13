package testutil

import (
	"testing"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/stretchr/testify/assert"
)

type SubscriptionMatcher struct {
	t    *testing.T
	want dal.Subscription
}

func NewSubscriptionMatcher(t *testing.T, want dal.Subscription) *SubscriptionMatcher {
	return &SubscriptionMatcher{
		t:    t,
		want: want,
	}
}

func (m SubscriptionMatcher) Matches(x interface{}) bool {
	actual, ok := x.(dal.Subscription)
	if !ok {
		m.t.Fatalf("SubscriptionMatcher.Matches: expected dal.Subscription, got %T", x)
		return false
	}

	m.want.CreatedAt = actual.CreatedAt
	return assert.Equal(m.t, m.want, actual)
}

func (m SubscriptionMatcher) String() string {
	return "SubscriptionMatcher.Matches"
}
