package calendar

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

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

// SyncConfig holds which statuses to sync and which group to use.
type SyncConfig struct {
	SyncOff   bool
	SyncMaybe bool
	SyncOn    bool
	Group     int // 1–12
}

// ShutdownsReader provides schedule and emergency state for calendar sync.
type ShutdownsReader interface {
	GetShutdowns(d dal.Date) (dal.Shutdowns, bool, error)
	GetEmergencyState() (dal.EmergencyState, error)
}

// Clock provides current time (e.g. for today/tomorrow and time window).
type Clock interface {
	Now() time.Time
}

// SyncService runs delete-then-recreate sync of power outage schedule to Google Calendar.
type SyncService struct {
	client *Client
	store  ShutdownsReader
	clock  Clock
	conf   SyncConfig
	loc    *time.Location
	log    *slog.Logger
}

// NewSyncService creates a calendar sync service.
func NewSyncService(client *Client, store ShutdownsReader, clock Clock, conf SyncConfig, loc *time.Location, log *slog.Logger) *SyncService {
	return &SyncService{
		client: client,
		store:  store,
		clock:  clock,
		conf:   conf,
		loc:    loc,
		log:    log.With("component", "calendar_sync"),
	}
}

// Sync performs full sync: skip if emergency, delete our events in [today, end of tomorrow], then create events from current schedule.
func (s *SyncService) Sync(ctx context.Context) error {
	state, err := s.store.GetEmergencyState()
	if err != nil {
		return fmt.Errorf("get emergency state: %w", err)
	}
	if state.Active {
		s.log.DebugContext(ctx, "Skipping calendar sync: emergency mode active")
		return nil
	}

	now := s.clock.Now().In(s.loc)
	today := dal.DateByTime(now)
	tomorrow := dal.TomorrowDateByTime(now)

	timeMin := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	timeMax := time.Date(tomorrow.Year, tomorrow.Month, tomorrow.Day, 23, 59, 59, 0, s.loc)

	s.log.InfoContext(ctx, "Starting calendar sync", "timeMin", timeMin.Format(time.RFC3339), "timeMax", timeMax.Format(time.RFC3339))

	ids, err := s.client.ListOurEvents(ctx, timeMin, timeMax)
	if err != nil {
		return fmt.Errorf("calendar sync failed: list: %w", err)
	}
	for _, id := range ids {
		if err := s.client.DeleteEvent(ctx, id); err != nil {
			return fmt.Errorf("calendar sync failed: delete %s: %w", id, err)
		}
	}
	s.log.DebugContext(ctx, "Deleted our events", "count", len(ids))

	todayShutdowns, hasToday, err := s.store.GetShutdowns(today)
	if err != nil {
		return fmt.Errorf("get shutdowns today: %w", err)
	}
	tomorrowShutdowns, hasTomorrow, err := s.store.GetShutdowns(tomorrow)
	if err != nil {
		return fmt.Errorf("get shutdowns tomorrow: %w", err)
	}

	var toCreate []eventPayload
	if hasToday {
		toCreate = append(toCreate, buildEventsFromSchedule(todayShutdowns, today, s.conf, s.loc)...)
	}
	if hasTomorrow {
		toCreate = append(toCreate, buildEventsFromSchedule(tomorrowShutdowns, tomorrow, s.conf, s.loc)...)
	}

	descBase := "SSO Notifier — power outage schedule"
	for _, ev := range toCreate {
		desc := descBase
		if ev.dateLabel != "" {
			desc = descBase + " — " + ev.dateLabel
		}
		_, err := s.client.InsertEvent(ctx, ev.summary, ev.startRFC3339, ev.endRFC3339, ev.colorID, desc)
		if err != nil {
			return fmt.Errorf("calendar sync failed: insert: %w", err)
		}
	}
	s.log.InfoContext(ctx, "Calendar sync completed", "deleted", len(ids), "created", len(toCreate))
	return nil
}

// CleanupStale deletes our events in the past lookbackDays (not including today).
// Window: [today - lookbackDays at 00:00, yesterday at 23:59:59]. Run periodically (e.g. every 6h).
func (s *SyncService) CleanupStale(ctx context.Context, lookbackDays int) error {
	now := s.clock.Now().In(s.loc)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	yesterdayEnd := todayStart.Add(-time.Second) // 23:59:59 yesterday
	timeMin := todayStart.AddDate(0, 0, -lookbackDays)

	s.log.InfoContext(ctx, "Starting calendar stale cleanup", "timeMin", timeMin.Format(time.RFC3339), "timeMax", yesterdayEnd.Format(time.RFC3339))

	ids, err := s.client.ListOurEvents(ctx, timeMin, yesterdayEnd)
	if err != nil {
		return fmt.Errorf("calendar cleanup failed: list: %w", err)
	}
	for _, id := range ids {
		if err := s.client.DeleteEvent(ctx, id); err != nil {
			return fmt.Errorf("calendar cleanup failed: delete %s: %w", id, err)
		}
	}
	s.log.InfoContext(ctx, "Calendar stale cleanup completed", "deleted", len(ids))
	return nil
}

type eventPayload struct {
	summary      string
	startRFC3339 string
	endRFC3339   string
	colorID      string
	dateLabel    string
}

// buildEventsFromSchedule returns event payloads for one day's schedule: one group, merged consecutive same-status, filtered by conf.
func buildEventsFromSchedule(shutdowns dal.Shutdowns, day dal.Date, conf SyncConfig, loc *time.Location) []eventPayload {
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
		startTime, errStart := parseTimeInDay(periods[i].From, day, loc)
		endTime, errEnd := parseTimeInDay(periods[i].To, day, loc)
		if errStart != nil || errEnd != nil {
			continue
		}
		out = append(out, eventPayload{
			summary:      summary,
			startRFC3339: startTime.Format(time.RFC3339),
			endRFC3339:   endTime.Format(time.RFC3339),
			colorID:      colorID,
			dateLabel:    shutdowns.Date,
		})
	}
	return out
}

func statusSyncEnabled(st dal.Status, conf SyncConfig) bool {
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

func summaryAndColorForStatus(st dal.Status) (summary, colorID string) {
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
func parseTimeInDay(s string, day dal.Date, loc *time.Location) (time.Time, error) {
	if s == "24:00" {
		startOfDay := time.Date(day.Year, day.Month, day.Day, 0, 0, 0, 0, loc)
		return startOfDay.Add(24 * time.Hour), nil //nolint:mnd // 24:00 = next day 00:00
	}
	t, err := time.ParseInLocation("15:04", s, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year, day.Month, day.Day, t.Hour(), t.Minute(), 0, 0, loc), nil
}
