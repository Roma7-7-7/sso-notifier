package service_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

const (
	summaryOff   = "Power off"
	summaryMaybe = "Possible outage"
	summaryOn    = "Power on"
)

func TestSyncService_SyncEvents(t *testing.T) {
	now := time.Date(2025, time.February, 12, 10, 0, 0, 0, time.UTC)
	today := dal.DateByTime(now)
	tomorrow := dal.TomorrowDateByTime(now)
	todayShutdowns := dal.Shutdowns{
		Date:    "12 лютого",
		Periods: []dal.Period{{From: "14:00", To: "14:30"}, {From: "14:30", To: "15:00"}},
		Groups: map[string]dal.ShutdownGroup{
			"4": {Number: 4, Items: []dal.Status{dal.OFF, dal.ON}},
		},
	}
	timeMin := time.Date(2025, time.February, 12, 0, 0, 0, 0, time.UTC)
	timeMax := time.Date(2025, time.February, 13, 23, 59, 59, 0, time.UTC)

	type fields struct {
		calendar func(*gomock.Controller) service.Calendar
		store    func(*gomock.Controller) service.ShutdownsStore
		now      time.Time
		conf     service.CalendarConfig
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "emergency_skips_sync",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					return mocks.NewMockCalendar(c)
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{Active: true}, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "no_schedule_skips_sync",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					return mocks.NewMockCalendar(c)
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(dal.Shutdowns{}, false, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "delete_then_create",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
						Return([]string{"old-id-1"}, nil)
					res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "old-id-1").Return(nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOn, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-2", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now: now,
				conf: service.CalendarConfig{
					CalendarID: "cal@test",
					SyncOff:    true,
					SyncOn:     true,
					Group:      4,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "get_emergency_state_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar { return mocks.NewMockCalendar(c) },
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, assert.AnError)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "get emergency state")
			},
		},
		{
			name: "get_shutdowns_today_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar { return mocks.NewMockCalendar(c) },
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(dal.Shutdowns{}, false, assert.AnError)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "get shutdowns today")
			},
		},
		{
			name: "get_shutdowns_tomorrow_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar { return mocks.NewMockCalendar(c) },
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, assert.AnError)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, SyncOn: true, Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "get shutdowns tomorrow")
			},
		},
		{
			name: "list_our_events_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
						Return(nil, assert.AnError)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "calendar sync failed: list")
			},
		},
		{
			name: "delete_event_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
						Return([]string{"old-id-1"}, nil)
					res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "old-id-1").Return(assert.AnError)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "delete old-id-1")
			},
		},
		{
			name: "insert_event_error",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("", assert.AnError)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "calendar sync failed: insert")
			},
		},
		{
			name: "group_not_in_schedule_deletes_only",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
						Return([]string{"old-id-1"}, nil)
					res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "old-id-1").Return(nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					// Schedule has only group "5", config asks for group 4
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "14:00", To: "15:00"}},
						Groups:  map[string]dal.ShutdownGroup{"5": {Number: 5, Items: []dal.Status{dal.OFF}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "sync_maybe_creates_possible_outage_event",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryMaybe, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "14:00", To: "14:30"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.MAYBE}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncMaybe: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "merged_consecutive_same_status_one_event",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "14:00", To: "14:30"}, {From: "14:30", To: "15:00"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.OFF, dal.OFF}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "tomorrow_only_sync",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					tomorrowShutdowns := dal.Shutdowns{
						Date:    "13 лютого",
						Periods: []dal.Period{{From: "09:00", To: "09:30"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.OFF}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(dal.Shutdowns{}, false, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(tomorrowShutdowns, true, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "period_ending_24_00",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "23:30", To: "24:00"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.OFF}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "unparseable_time_skipped",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					res.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
						Return("new-id-1", nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "14:00", To: "14:30"}, {From: "xx:00", To: "15:00"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.OFF, dal.OFF}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "unknown_status_skipped",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{{From: "14:00", To: "14:30"}},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.Status("X")}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
		{
			name: "empty_periods_no_events_created",
			fields: fields{
				calendar: func(c *gomock.Controller) service.Calendar {
					res := mocks.NewMockCalendar(c)
					return res
				},
				store: func(c *gomock.Controller) service.ShutdownsStore {
					sh := dal.Shutdowns{
						Date:    "12 лютого",
						Periods: []dal.Period{},
						Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{}}},
					}
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
					res.EXPECT().GetShutdowns(today).Return(sh, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				now:  now,
				conf: service.CalendarConfig{CalendarID: "cal@test", SyncOff: true, Group: 4},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cal := tt.fields.calendar(ctrl)
			store := tt.fields.store(ctrl)
			clk := clock.NewMock(tt.fields.now)
			svc := service.NewCalendarService(tt.fields.conf, cal, store, clk, slog.New(slog.DiscardHandler))
			tt.wantErr(t, svc.SyncEvents(context.Background()), "SyncEvents(_)")
		})
	}
}

func TestCalendarService_CleanupStaleEvents(t *testing.T) {
	now := time.Date(2025, time.February, 12, 10, 0, 0, 0, time.UTC)
	timeMin := time.Date(2025, time.February, 5, 0, 0, 0, 0, time.UTC)     // today - 7
	timeMax := time.Date(2025, time.February, 11, 23, 59, 59, 0, time.UTC) // yesterday 23:59:59
	lookbackDays := 7
	conf := service.CalendarConfig{CalendarID: "cal@test"}

	tests := []struct {
		name     string
		calendar func(*gomock.Controller) service.Calendar
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name: "success_no_events",
			calendar: func(c *gomock.Controller) service.Calendar {
				res := mocks.NewMockCalendar(c)
				res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil)
				return res
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_deletes_events",
			calendar: func(c *gomock.Controller) service.Calendar {
				res := mocks.NewMockCalendar(c)
				res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
					Return([]string{"id-1", "id-2"}, nil)
				res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "id-1").Return(nil)
				res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "id-2").Return(nil)
				return res
			},
			wantErr: assert.NoError,
		},
		{
			name: "list_error",
			calendar: func(c *gomock.Controller) service.Calendar {
				res := mocks.NewMockCalendar(c)
				res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
					Return(nil, assert.AnError)
				return res
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "calendar cleanup failed: list")
			},
		},
		{
			name: "delete_error",
			calendar: func(c *gomock.Controller) service.Calendar {
				res := mocks.NewMockCalendar(c)
				res.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).
					Return([]string{"id-1"}, nil)
				res.EXPECT().DeleteEvent(gomock.Any(), "cal@test", "id-1").Return(assert.AnError)
				return res
			},
			wantErr: func(t assert.TestingT, err error, _ ...any) bool {
				return assert.Error(t, err) && assert.Contains(t, err.Error(), "calendar cleanup failed: delete id-1")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			cal := tt.calendar(ctrl)
			store := mocks.NewMockShutdownsStore(ctrl)
			clk := clock.NewMock(now)
			svc := service.NewCalendarService(conf, cal, store, clk, slog.New(slog.DiscardHandler))
			tt.wantErr(t, svc.CleanupStaleEvents(context.Background(), lookbackDays), "CleanupStaleEvents(_, %d)", lookbackDays)
		})
	}
}

// TestCalendarService_SyncEvents_skips_when_schedule_hash_unchanged verifies that a second SyncEvents
// call with the same schedule does not list/delete/insert calendar events (cache prevents recreation).
func TestCalendarService_SyncEvents_skips_when_schedule_hash_unchanged(t *testing.T) {
	now := time.Date(2025, time.February, 12, 10, 0, 0, 0, time.UTC)
	today := dal.DateByTime(now)
	tomorrow := dal.TomorrowDateByTime(now)
	todayShutdowns := dal.Shutdowns{
		Date:    "12 лютого",
		Periods: []dal.Period{{From: "14:00", To: "14:30"}, {From: "14:30", To: "15:00"}},
		Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: []dal.Status{dal.OFF, dal.ON}}},
	}
	timeMin := time.Date(2025, time.February, 12, 0, 0, 0, 0, time.UTC)
	timeMax := time.Date(2025, time.February, 13, 23, 59, 59, 0, time.UTC)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cal := mocks.NewMockCalendar(ctrl)
	cal.EXPECT().ListOurEvents(gomock.Any(), "cal@test", timeMin, timeMax).Return([]string{}, nil).Times(1)
	cal.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOff, gomock.Any(), gomock.Any(), gomock.Any()).
		Return("id-1", nil).Times(1)
	cal.EXPECT().InsertEvent(gomock.Any(), "cal@test", summaryOn, gomock.Any(), gomock.Any(), gomock.Any()).
		Return("id-2", nil).Times(1)

	store := mocks.NewMockShutdownsStore(ctrl)
	store.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil).Times(2)
	store.EXPECT().GetShutdowns(today).Return(todayShutdowns, true, nil).Times(2)
	store.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil).Times(2)

	conf := service.CalendarConfig{
		CalendarID: "cal@test",
		SyncOff:    true,
		SyncOn:     true,
		Group:      4,
	}
	svc := service.NewCalendarService(conf, cal, store, clock.NewMock(now), slog.New(slog.DiscardHandler))

	err := svc.SyncEvents(context.Background())
	assert.NoError(t, err)

	err = svc.SyncEvents(context.Background())
	assert.NoError(t, err)
}
