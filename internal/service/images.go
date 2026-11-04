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

func (i ScheduleImage) segments() ([]render.Segment, error) {
	if len(i.Statuses) != len(i.Periods) {
		return nil, fmt.Errorf("statuses count %d does not match periods count %d", len(i.Statuses), len(i.Periods))
	}
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

// cacheKey identifies images with identical content, so one render can serve many subscribers.
func (i ScheduleImage) cacheKey() string {
	var sb strings.Builder
	sb.WriteString(i.Date + "|" + i.GroupNum)
	for idx, p := range i.Periods {
		if idx < len(i.Statuses) {
			fmt.Fprintf(&sb, "|%s-%s=%s", p.From, p.To, i.Statuses[idx])
		}
	}
	return sb.String()
}

// Day converts the schedule to renderer input.
func (i ScheduleImage) Day() (render.Day, error) {
	segments, err := i.segments()
	if err != nil {
		return render.Day{}, fmt.Errorf("build segments: %w", err)
	}
	var offMin, maybeMin int
	for _, s := range segments {
		switch s.Kind {
		case render.KindOff:
			offMin += s.ToMin - s.FromMin
		case render.KindMaybe:
			maybeMin += s.ToMin - s.FromMin
		case render.KindOn:
		}
	}
	return render.Day{
		Title:    "Група " + i.GroupNum,
		Subtitle: fmt.Sprintf("%s · без світла %s · можливо %s", i.Date, formatDuration(offMin), formatDuration(maybeMin)),
		Segments: segments,
	}, nil
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
	return hours*60 + minutes, nil
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
