package scheduleapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

const (
	StatusOn    = "ON"
	StatusOff   = "OFF"
	StatusMaybe = "MAYBE"
)

type Interval struct {
	Start  time.Time `json:"start"`
	End    time.Time `json:"end"`
	Status string    `json:"status"`
}

// toIntervals converts a day's per-period statuses into time ranges per group.
func toIntervals(day dal.Shutdowns, loc *time.Location) (map[string][]Interval, error) {
	date, err := time.ParseInLocation("02.01.2006", day.Date, loc)
	if err != nil {
		return nil, fmt.Errorf("parse date %q: %w", day.Date, err)
	}

	res := make(map[string][]Interval, len(day.Groups))
	for key, g := range day.Groups {
		if len(g.Items) != len(day.Periods) {
			return nil, fmt.Errorf("group %s: items=%d but periods=%d", key, len(g.Items), len(day.Periods))
		}

		intervals := make([]Interval, 0, len(g.Items))
		for i, st := range g.Items {
			start, err := atClock(date, day.Periods[i].From)
			if err != nil {
				return nil, fmt.Errorf("group %s period %d: %w", key, i, err)
			}
			end, err := atClock(date, day.Periods[i].To)
			if err != nil {
				return nil, fmt.Errorf("group %s period %d: %w", key, i, err)
			}
			intervals = append(intervals, Interval{Start: start, End: end, Status: statusName(st)})
		}
		res[key] = intervals
	}

	return res, nil
}

// atClock builds a time for "HH:MM" on the given date; "24:00" rolls over to the next day.
func atClock(date time.Time, clock string) (time.Time, error) {
	h, m, ok := strings.Cut(clock, ":")
	if !ok {
		return time.Time{}, fmt.Errorf("invalid clock %q", clock)
	}
	hours, err := strconv.Atoi(h)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid clock %q: %w", clock, err)
	}
	minutes, err := strconv.Atoi(m)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid clock %q: %w", clock, err)
	}

	return time.Date(date.Year(), date.Month(), date.Day(), hours, minutes, 0, 0, date.Location()), nil
}

func statusName(s dal.Status) string {
	switch s {
	case dal.ON:
		return StatusOn
	case dal.OFF:
		return StatusOff
	case dal.MAYBE:
		return StatusMaybe
	default:
		return StatusMaybe
	}
}

// merge joins adjacent intervals with the same status, including across midnight.
func merge(intervals []Interval) []Interval {
	res := make([]Interval, 0, len(intervals))
	for _, iv := range intervals {
		if n := len(res); n > 0 && res[n-1].Status == iv.Status && res[n-1].End.Equal(iv.Start) {
			res[n-1].End = iv.End
			continue
		}
		res = append(res, iv)
	}

	return res
}
