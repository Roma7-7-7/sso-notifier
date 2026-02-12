package calendar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

func TestJoinPeriods(t *testing.T) {
	tests := []struct {
		name     string
		periods  []dal.Period
		statuses []dal.Status
		wantP    []dal.Period
		wantS    []dal.Status
	}{
		{
			name:     "empty",
			periods:  nil,
			statuses: nil,
			wantP:    nil,
			wantS:    nil,
		},
		{
			name:     "single",
			periods:  []dal.Period{{From: "10:00", To: "10:30"}},
			statuses: []dal.Status{dal.OFF},
			wantP:    []dal.Period{{From: "10:00", To: "10:30"}},
			wantS:    []dal.Status{dal.OFF},
		},
		{
			name: "merge consecutive same status",
			periods: []dal.Period{
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
				{From: "11:00", To: "11:30"},
			},
			statuses: []dal.Status{dal.OFF, dal.OFF, dal.OFF},
			wantP:    []dal.Period{{From: "10:00", To: "11:30"}},
			wantS:    []dal.Status{dal.OFF},
		},
		{
			name: "different statuses not merged",
			periods: []dal.Period{
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
				{From: "11:00", To: "11:30"},
			},
			statuses: []dal.Status{dal.OFF, dal.ON, dal.MAYBE},
			wantP: []dal.Period{
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
				{From: "11:00", To: "11:30"},
			},
			wantS: []dal.Status{dal.OFF, dal.ON, dal.MAYBE},
		},
		{
			name: "merge pairs",
			periods: []dal.Period{
				{From: "09:00", To: "09:30"},
				{From: "09:30", To: "10:00"},
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
			},
			statuses: []dal.Status{dal.OFF, dal.OFF, dal.ON, dal.ON},
			wantP: []dal.Period{
				{From: "09:00", To: "10:00"},
				{From: "10:00", To: "11:00"},
			},
			wantS: []dal.Status{dal.OFF, dal.ON},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotP, gotS := joinPeriods(tt.periods, tt.statuses)
			assert.Equal(t, tt.wantP, gotP)
			assert.Equal(t, tt.wantS, gotS)
		})
	}
}

func TestBuildEventsFromSchedule(t *testing.T) {
	loc := mustKyiv(t)
	day := dal.Date{Year: 2025, Month: time.February, Day: 12}

	t.Run("empty groups", func(t *testing.T) {
		shutdowns := dal.Shutdowns{
			Date:    "12 лютого",
			Periods: []dal.Period{{From: "10:00", To: "11:00"}},
			Groups:  map[string]dal.ShutdownGroup{},
		}
		conf := SyncConfig{SyncOff: true, Group: 4}
		got := buildEventsFromSchedule(shutdowns, day, conf, loc)
		assert.Empty(t, got)
	})

	t.Run("group missing", func(t *testing.T) {
		shutdowns := dal.Shutdowns{
			Date:    "12 лютого",
			Periods: []dal.Period{{From: "10:00", To: "11:00"}},
			Groups:  map[string]dal.ShutdownGroup{"1": {Number: 1, Items: []dal.Status{dal.OFF}}},
		}
		conf := SyncConfig{SyncOff: true, Group: 4}
		got := buildEventsFromSchedule(shutdowns, day, conf, loc)
		assert.Empty(t, got)
	})

	t.Run("sync only OFF", func(t *testing.T) {
		shutdowns := dal.Shutdowns{
			Date: "12 лютого",
			Periods: []dal.Period{
				{From: "10:00", To: "10:30"},
				{From: "10:30", To: "11:00"},
				{From: "11:00", To: "11:30"},
			},
			Groups: map[string]dal.ShutdownGroup{
				"4": {
					Number: 4,
					Items:  []dal.Status{dal.OFF, dal.OFF, dal.ON},
				},
			},
		}
		conf := SyncConfig{SyncOff: true, SyncMaybe: false, SyncOn: false, Group: 4}
		got := buildEventsFromSchedule(shutdowns, day, conf, loc)
		require.Len(t, got, 1)
		assert.Equal(t, summaryOff, got[0].summary)
		assert.Equal(t, colorIDOff, got[0].colorID)
		assert.Equal(t, "12 лютого", got[0].dateLabel)
		// 10:00-11:00 merged (two OFF slots)
		assert.Contains(t, got[0].startRFC3339, "T10:00:")
		assert.Contains(t, got[0].endRFC3339, "T11:00:")
	})

	t.Run("all statuses when all sync enabled", func(t *testing.T) {
		shutdowns := dal.Shutdowns{
			Date: "12 лютого",
			Periods: []dal.Period{
				{From: "14:00", To: "14:30"},
				{From: "14:30", To: "15:00"},
				{From: "15:00", To: "15:30"},
			},
			Groups: map[string]dal.ShutdownGroup{
				"4": {
					Number: 4,
					Items:  []dal.Status{dal.OFF, dal.MAYBE, dal.ON},
				},
			},
		}
		conf := SyncConfig{SyncOff: true, SyncMaybe: true, SyncOn: true, Group: 4}
		got := buildEventsFromSchedule(shutdowns, day, conf, loc)
		require.Len(t, got, 3)
		assert.Equal(t, summaryOff, got[0].summary)
		assert.Equal(t, summaryMaybe, got[1].summary)
		assert.Equal(t, summaryOn, got[2].summary)
		assert.Equal(t, colorIDOff, got[0].colorID)
		assert.Equal(t, colorIDMaybe, got[1].colorID)
		assert.Equal(t, colorIDOn, got[2].colorID)
	})

	t.Run("RFC3339 in Europe/Kyiv", func(t *testing.T) {
		shutdowns := dal.Shutdowns{
			Date:    "12 лютого",
			Periods: []dal.Period{{From: "09:00", To: "09:30"}},
			Groups: map[string]dal.ShutdownGroup{
				"4": {Number: 4, Items: []dal.Status{dal.OFF}},
			},
		}
		conf := SyncConfig{SyncOff: true, Group: 4}
		got := buildEventsFromSchedule(shutdowns, day, conf, loc)
		require.Len(t, got, 1)
		_, err := time.Parse(time.RFC3339, got[0].startRFC3339)
		require.NoError(t, err)
		_, err = time.Parse(time.RFC3339, got[0].endRFC3339)
		require.NoError(t, err)
	})
}

func TestSummaryAndColorForStatus(t *testing.T) {
	tests := []struct {
		status dal.Status
		sum    string
		color  string
	}{
		{dal.OFF, summaryOff, colorIDOff},
		{dal.MAYBE, summaryMaybe, colorIDMaybe},
		{dal.ON, summaryOn, colorIDOn},
		{dal.Status("?"), "", ""},
	}
	for _, tt := range tests {
		s, c := summaryAndColorForStatus(tt.status)
		assert.Equal(t, tt.sum, s, "status %s", tt.status)
		assert.Equal(t, tt.color, c, "status %s", tt.status)
	}
}

func TestStatusSyncEnabled(t *testing.T) {
	conf := SyncConfig{SyncOff: true, SyncMaybe: false, SyncOn: true}
	assert.True(t, statusSyncEnabled(dal.OFF, conf))
	assert.False(t, statusSyncEnabled(dal.MAYBE, conf))
	assert.True(t, statusSyncEnabled(dal.ON, conf))
	assert.False(t, statusSyncEnabled(dal.Status("x"), conf))
}

func TestParseTimeInDay(t *testing.T) {
	loc := mustKyiv(t)
	day := dal.Date{Year: 2025, Month: time.June, Day: 15}

	got, err := parseTimeInDay("14:30", day, loc)
	require.NoError(t, err)
	assert.Equal(t, 2025, got.Year())
	assert.Equal(t, time.June, got.Month())
	assert.Equal(t, 15, got.Day())
	assert.Equal(t, 14, got.Hour())
	assert.Equal(t, 30, got.Minute())
	assert.Equal(t, loc, got.Location())

	_, err = parseTimeInDay("25:00", day, loc)
	assert.Error(t, err)
}

func TestParseTimeInDay_24_00(t *testing.T) {
	loc := mustKyiv(t)
	day := dal.Date{Year: 2025, Month: time.June, Day: 15}

	got, err := parseTimeInDay("24:00", day, loc)
	require.NoError(t, err)
	// 24:00 = midnight next day
	assert.Equal(t, 2025, got.Year())
	assert.Equal(t, time.June, got.Month())
	assert.Equal(t, 16, got.Day())
	assert.Equal(t, 0, got.Hour())
	assert.Equal(t, 0, got.Minute())
}

func TestBuildEventsFromSchedule_LastInterval24_00(t *testing.T) {
	loc := mustKyiv(t)
	day := dal.Date{Year: 2025, Month: time.February, Day: 12}
	// Day that ends with OFF 23:30–24:00
	shutdowns := dal.Shutdowns{
		Date: "12 лютого",
		Periods: []dal.Period{
			{From: "23:00", To: "23:30"},
			{From: "23:30", To: "24:00"},
		},
		Groups: map[string]dal.ShutdownGroup{
			"4": {Number: 4, Items: []dal.Status{dal.ON, dal.OFF}},
		},
	}
	conf := SyncConfig{SyncOff: true, SyncOn: true, Group: 4}
	got := buildEventsFromSchedule(shutdowns, day, conf, loc)
	require.Len(t, got, 2)
	// First event: ON 23:00–23:30
	assert.Equal(t, summaryOn, got[0].summary)
	assert.Contains(t, got[0].endRFC3339, "T23:30:")
	// Second event: OFF 23:30–24:00 (last interval; 24:00 must be included)
	assert.Equal(t, summaryOff, got[1].summary)
	assert.Contains(t, got[1].startRFC3339, "T23:30:")
	assert.Contains(t, got[1].endRFC3339, "T00:00:00") // next day 00:00
	assert.Contains(t, got[1].endRFC3339, "2025-02-13")
}

func mustKyiv(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Kyiv")
	require.NoError(t, err)
	return loc
}
