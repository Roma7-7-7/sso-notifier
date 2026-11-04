package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/render"
)

// ScheduleImage is the full-day schedule of one group on one date, to be drawn as an image.
type ScheduleImage struct {
	Date     string
	GroupNum string
	Periods  []dal.Period
	Statuses []dal.Status
}

type dayWindows struct {
	off, maybe []render.Segment
}

func (i ScheduleImage) segments() ([]render.Segment, error) {
	segments := make([]render.Segment, 0, len(i.Periods))
	for idx, p := range i.Periods {
		from, err := parseMinutes(p.From)
		if err != nil {
			return nil, fmt.Errorf("parse period start: %w", err)
		}
		to, err := parseMinutes(p.To)
		if err != nil {
			return nil, fmt.Errorf("parse period end: %w", err)
		}
		kind, ok := kindForStatus(i.Statuses[idx])
		if !ok {
			return nil, fmt.Errorf("unknown status %q", i.Statuses[idx])
		}
		segments = append(segments, render.Segment{FromMin: from, ToMin: to, Kind: kind})
	}
	return segments, nil
}

// Day converts the schedule to renderer input.
func (i ScheduleImage) Day() (render.Day, error) {
	segments, err := i.segments()
	if err != nil {
		return render.Day{}, fmt.Errorf("build segments: %w", err)
	}
	w := splitWindows(segments)
	return render.Day{
		Title:    "Група " + i.GroupNum,
		Subtitle: fmt.Sprintf("%s · без світла %s · можливо %s", i.Date, formatDuration(totalMinutes(w.off)), formatDuration(totalMinutes(w.maybe))),
		Segments: segments,
	}, nil
}

func splitWindows(segments []render.Segment) dayWindows {
	var w dayWindows
	for _, s := range segments {
		switch s.Kind {
		case render.KindOff:
			w.off = append(w.off, s)
		case render.KindMaybe:
			w.maybe = append(w.maybe, s)
		case render.KindOn:
		}
	}
	return w
}

func totalMinutes(segments []render.Segment) int {
	total := 0
	for _, s := range segments {
		total += s.ToMin - s.FromMin
	}
	return total
}

func kindForStatus(s dal.Status) (render.Kind, bool) {
	switch s {
	case dal.ON:
		return render.KindOn, true
	case dal.OFF:
		return render.KindOff, true
	case dal.MAYBE:
		return render.KindMaybe, true
	default:
		return 0, false
	}
}

func parseMinutes(hhmm string) (int, error) {
	h, m, ok := strings.Cut(hhmm, ":")
	if !ok {
		return 0, fmt.Errorf("invalid time %q", hhmm)
	}
	hours, err := strconv.Atoi(h)
	if err != nil {
		return 0, fmt.Errorf("invalid hours in %q: %w", hhmm, err)
	}
	minutes, err := strconv.Atoi(m)
	if err != nil {
		return 0, fmt.Errorf("invalid minutes in %q: %w", hhmm, err)
	}
	return hours*60 + minutes, nil //nolint:mnd // minutes per hour
}

func formatDuration(minutes int) string {
	h, m := minutes/60, minutes%60 //nolint:mnd // minutes per hour
	switch {
	case h == 0 && m == 0:
		return "0хв"
	case h == 0:
		return fmt.Sprintf("%dхв", m)
	case m == 0:
		return fmt.Sprintf("%dг", h)
	default:
		return fmt.Sprintf("%dг %dхв", h, m)
	}
}
