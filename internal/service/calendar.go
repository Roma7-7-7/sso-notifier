package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/calendar"
	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

//go:generate go run go.uber.org/mock/mockgen@latest -package mocks -destination mocks/calendar.go . Calendar

// Calendar event color IDs (Google Calendar palette)
const (
	colorIDOff   = "11" // Tomato — red
	colorIDMaybe = "5"  // Banana — yellow
	colorIDOn    = "10" // Basil — green
)

// Event summary strings (English, per design doc)
const (
	summaryOff   = "Power off"
	summaryMaybe = "Possible outage"
	summaryOn    = "Power on"
)

// CalendarConfig holds which statuses to sync and which group to use.
type CalendarConfig struct {
	CalendarID string
	SyncOff    bool
	SyncMaybe  bool
	SyncOn     bool
	Group      int // 1–12
}

type Calendar interface {
	ListOurEvents(ctx context.Context, calendarID string, timeMin, timeMax time.Time) ([]string, error)
	InsertEvent(ctx context.Context, calendarID, summary string, start, end time.Time, params calendar.EventParams) (string, error)
	DeleteEvent(ctx context.Context, calendarID, eventID string) error
}

// CalendarService runs delete-then-recreate sync of power outage schedule to Google Calendar.
type CalendarService struct {
	calendar Calendar
	store    ShutdownsStore
	clock    Clock
	conf     CalendarConfig

	todayCache    string
	tomorrowCache string

	mx  sync.Mutex
	log *slog.Logger
}

// NewCalendarService creates a calendar sync service.
func NewCalendarService(conf CalendarConfig, calendar Calendar, store ShutdownsStore, clock Clock, log *slog.Logger) *CalendarService {
	todayCache := atomic.Value{}
	tomorrowCache := atomic.Value{}

	todayCache.Store("")
	tomorrowCache.Store("")

	return &CalendarService{
		calendar: calendar,
		store:    store,
		clock:    clock,
		conf:     conf,
		mx:       sync.Mutex{},
		log:      log.With("component", "calendar_sync"),
	}
}

// SyncEvents performs full sync: skip if emergency, delete our events in [today, end of tomorrow], then create events from current schedule.
func (s *CalendarService) SyncEvents(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	state, err := s.store.GetEmergencyState()
	if err != nil {
		return fmt.Errorf("get emergency state: %w", err)
	}
	if state.Active {
		s.log.DebugContext(ctx, "Skipping calendar sync: emergency mode active")
		return nil
	}

	now := s.clock.Now()
	today := dal.DateByTime(now)
	tomorrow := dal.TomorrowDateByTime(now)

	timeMin := s.clock.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0)
	timeMax := s.clock.Date(tomorrow.Year, tomorrow.Month, tomorrow.Day, 23, 59, 59, 0) //nolint:mnd // it's ok

	s.log.InfoContext(ctx, "Starting calendar sync", "timeMin", timeMin.Format(time.RFC3339), "timeMax", timeMax.Format(time.RFC3339))

	todayShutdowns, hasToday, err := s.store.GetShutdowns(today)
	if err != nil {
		return fmt.Errorf("get shutdowns today: %w", err)
	}
	tomorrowShutdowns, hasTomorrow, err := s.store.GetShutdowns(tomorrow)
	if err != nil {
		return fmt.Errorf("get shutdowns tomorrow: %w", err)
	}

	groupNum := strconv.Itoa(s.conf.Group)
	if !hasToday && !hasTomorrow {
		s.log.WarnContext(ctx, "Skipping calendar sync: no today or tomorrow schedule")
		return nil
	}
	todayGroup, todayOk := todayShutdowns.Groups[groupNum]
	newTodayHash := shutdownGroupHash(todayGroup)
	tomorrowGroup, tomorrowOk := tomorrowShutdowns.Groups[groupNum]
	newTomorrowHash := shutdownGroupHash(tomorrowGroup)
	if (todayOk && s.todayCache == newTodayHash) && (!tomorrowOk || s.tomorrowCache == newTomorrowHash) {
		s.log.DebugContext(ctx, "Skipping calendar sync: today and tomorrow schedules not changed")
		return nil
	}

	ids, err := s.cleanupEvents(ctx, timeMin, timeMax)
	if err != nil {
		return err
	}
	s.log.DebugContext(ctx, "Deleted our events", "count", len(ids))

	var toCreate []eventPayload
	if hasToday {
		toCreate = append(toCreate, buildEventsFromSchedule(todayShutdowns, today, s.conf, s.clock)...)
	}
	if hasTomorrow {
		toCreate = append(toCreate, buildEventsFromSchedule(tomorrowShutdowns, tomorrow, s.conf, s.clock)...)
	}

	if err = s.createEvents(ctx, toCreate); err != nil {
		return err
	}

	s.todayCache = newTodayHash
	s.tomorrowCache = newTomorrowHash

	s.log.InfoContext(ctx, "Calendar sync completed", "deleted", len(ids), "created", len(toCreate))
	return nil
}

func (s *CalendarService) cleanupEvents(ctx context.Context, timeMin time.Time, timeMax time.Time) ([]string, error) {
	ids, err := s.calendar.ListOurEvents(ctx, s.conf.CalendarID, timeMin, timeMax)
	if err != nil {
		return nil, fmt.Errorf("calendar sync failed: list: %w", err)
	}
	for _, id := range ids {
		if err := s.calendar.DeleteEvent(ctx, s.conf.CalendarID, id); err != nil {
			return nil, fmt.Errorf("calendar sync failed: delete %s: %w", id, err)
		}
	}
	return ids, nil
}

func (s *CalendarService) createEvents(ctx context.Context, toCreate []eventPayload) error {
	descBase := "SSO Notifier — power outage schedule"
	for _, ev := range toCreate {
		desc := descBase
		if ev.dateLabel != "" {
			desc = descBase + " — " + ev.dateLabel
		}
		_, err := s.calendar.InsertEvent(ctx, s.conf.CalendarID, ev.summary, ev.start, ev.end, calendar.EventParams{
			ColorID:     ev.colorID,
			Description: desc,
		})
		if err != nil {
			return fmt.Errorf("calendar sync failed: insert: %w", err)
		}
	}
	return nil
}

// CleanupStaleEvents deletes our events in the past lookbackDays (not including today).
// Window: [today - lookbackDays at 00:00, yesterday at 23:59:59]. Run periodically (e.g. every 6h).
func (s *CalendarService) CleanupStaleEvents(ctx context.Context, lookbackDays int) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	now := s.clock.Now()
	todayStart := s.clock.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0)
	yesterdayEnd := todayStart.Add(-time.Second) // 23:59:59 yesterday
	timeMin := todayStart.AddDate(0, 0, -lookbackDays)

	s.log.InfoContext(ctx, "Starting calendar stale cleanup", "timeMin", timeMin.Format(time.RFC3339), "timeMax", yesterdayEnd.Format(time.RFC3339))

	ids, err := s.calendar.ListOurEvents(ctx, s.conf.CalendarID, timeMin, yesterdayEnd)
	if err != nil {
		return fmt.Errorf("calendar cleanup failed: list: %w", err)
	}
	for _, id := range ids {
		if err := s.calendar.DeleteEvent(ctx, s.conf.CalendarID, id); err != nil {
			return fmt.Errorf("calendar cleanup failed: delete %s: %w", id, err)
		}
	}
	s.log.InfoContext(ctx, "Calendar stale cleanup completed", "deleted", len(ids))
	return nil
}

type eventPayload struct {
	summary   string
	start     time.Time
	end       time.Time
	colorID   string
	dateLabel string
}

// buildEventsFromSchedule returns event payloads for one day's schedule: one group, merged consecutive same-status, filtered by conf.
func buildEventsFromSchedule(shutdowns dal.Shutdowns, day dal.Date, conf CalendarConfig, clock Clock) []eventPayload {
	groupKey := strconv.Itoa(conf.Group)
	group, ok := shutdowns.Groups[groupKey]
	if !ok {
		return nil
	}
	periods, statuses := joinPeriods(shutdowns.Periods, group.Items)
	var out []eventPayload
	for i, st := range statuses {
		if !statusSyncEnabled(st, conf) {
			continue
		}
		summary, colorID := summaryAndColorForStatus(st)
		startTime, errStart := parseTimeInDay(periods[i].From, day, clock)
		endTime, errEnd := parseTimeInDay(periods[i].To, day, clock)
		if errStart != nil || errEnd != nil {
			continue
		}
		out = append(out, eventPayload{
			summary:   summary,
			start:     startTime,
			end:       endTime,
			colorID:   colorID,
			dateLabel: shutdowns.Date,
		})
	}
	return out
}

func statusSyncEnabled(st dal.Status, conf CalendarConfig) bool {
	switch st {
	case dal.OFF:
		return conf.SyncOff
	case dal.MAYBE:
		return conf.SyncMaybe
	case dal.ON:
		return conf.SyncOn
	default:
		return false
	}
}

func summaryAndColorForStatus(st dal.Status) (string, string) {
	switch st {
	case dal.OFF:
		return summaryOff, colorIDOff
	case dal.MAYBE:
		return summaryMaybe, colorIDMaybe
	case dal.ON:
		return summaryOn, colorIDOn
	default:
		return "", ""
	}
}

// joinPeriods merges consecutive periods with the same status (same logic as service/messages.join).
func joinPeriods(periods []dal.Period, statuses []dal.Status) ([]dal.Period, []dal.Status) {
	if len(periods) == 0 {
		return nil, nil
	}
	var mergedP []dal.Period
	var mergedS []dal.Status
	curFrom, curTo := periods[0].From, periods[0].To
	curSt := statuses[0]
	for i := 1; i < len(periods); i++ {
		if statuses[i] == curSt {
			curTo = periods[i].To
			continue
		}
		mergedP = append(mergedP, dal.Period{From: curFrom, To: curTo})
		mergedS = append(mergedS, curSt)
		curFrom, curTo = periods[i].From, periods[i].To
		curSt = statuses[i]
	}
	mergedP = append(mergedP, dal.Period{From: curFrom, To: curTo})
	mergedS = append(mergedS, curSt)
	return mergedP, mergedS
}

// parseTimeInDay parses a "15:04" time string and returns that time on the given day in loc.
// "24:00" is treated as midnight at the start of the next day (end-of-day).
func parseTimeInDay(s string, day dal.Date, clock Clock) (time.Time, error) {
	if s == "24:00" {
		startOfDay := clock.Date(day.Year, day.Month, day.Day, 0, 0, 0, 0)
		return startOfDay.Add(24 * time.Hour), nil //nolint:mnd // 24:00 = next day 00:00
	}
	t, err := clock.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %q: %w", s, err)
	}
	return clock.Date(day.Year, day.Month, day.Day, t.Hour(), t.Minute(), 0, 0), nil
}
