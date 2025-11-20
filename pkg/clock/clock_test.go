package clock_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func TestClock_Now(t *testing.T) {
	c := clock.New()
	require.NotNil(t, c)

	startAt := time.Now()
	now := c.Now()
	assert.GreaterOrEqual(t, now, startAt)
	assert.NotEqual(t, time.UTC, now.Location())

	time.Sleep(time.Second)
	startAt = time.Now()
	now = c.Now()
	assert.GreaterOrEqual(t, now, startAt)
	assert.NotEqual(t, time.UTC, now.Location())

	c = clock.NewWithLocation(time.UTC)
	require.NotNil(t, c)

	startAt = time.Now()
	now = c.Now()
	assert.GreaterOrEqual(t, now, startAt)
	assert.Equal(t, time.UTC, now.Location())

	time.Sleep(time.Second)
	startAt = time.Now()
	now = c.Now()
	assert.GreaterOrEqual(t, now, startAt)
	assert.Equal(t, time.UTC, now.Location())
}

func TestMock_Now(t *testing.T) {
	m := clock.NewMock(time.Date(2025, time.November, 20, 17, 7, 0, 0, time.UTC))
	require.NotNil(t, m)

	assert.Equal(t, time.Date(2025, time.November, 20, 17, 7, 0, 0, time.UTC), m.Now())
	assert.Equal(t, time.Date(2025, time.November, 20, 17, 7, 0, 0, time.UTC), m.Now())

	m.Set(time.Date(2025, time.November, 21, 17, 7, 0, 0, time.UTC))
	assert.Equal(t, time.Date(2025, time.November, 21, 17, 7, 0, 0, time.UTC), m.Now())

	m = clock.NewMockF(func() time.Time {
		return time.Date(2025, time.November, 22, 17, 7, 0, 0, time.UTC)
	})
	require.NotNil(t, m)

	assert.Equal(t, time.Date(2025, time.November, 22, 17, 7, 0, 0, time.UTC), m.Now())
	assert.Equal(t, time.Date(2025, time.November, 22, 17, 7, 0, 0, time.UTC), m.Now())

	m.SetF(func() time.Time {
		return time.Date(2025, time.November, 23, 17, 7, 0, 0, time.UTC)
	})
	assert.Equal(t, time.Date(2025, time.November, 23, 17, 7, 0, 0, time.UTC), m.Now())
	assert.Equal(t, time.Date(2025, time.November, 23, 17, 7, 0, 0, time.UTC), m.Now())
}
