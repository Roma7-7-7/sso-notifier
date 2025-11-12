package service

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"text/template"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

// messageTemplate is the main notification template
// IMPORTANT: If you change this template or the rendering logic below, you must also update:
// - internal/service/TEMPLATES.md - Update examples and documentation
// - CLAUDE.md - Update message format examples in "PowerSupplyMessage Templates" section
//
//nolint:gochecknoglobals // it's template
var messageTemplate = template.Must(template.New("message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}:
{{range .StatusLines}}{{if .Periods}}  {{.Emoji}} {{.Label}}:{{range .Periods}} {{.From}} - {{.To}};{{end}}
{{end}}{{end}}
{{end}}{{end}}`))

type (
	PowerSupplyMessage struct {
		Text                  string
		TodayUpdatedGroups    map[string]string // groupNum -> newHash for today
		TomorrowUpdatedGroups map[string]string // groupNum -> newHash for tomorrow (if applicable)
	}

	PowerSupplyScheduleMessageBuilder struct {
		shutdowns    dal.Shutdowns
		nextDayTable *dal.Shutdowns // Optional next day shutdowns
		now          time.Time
	}
)

func NewPowerSupplyScheduleMessageBuilder(shutdowns dal.Shutdowns, now time.Time) *PowerSupplyScheduleMessageBuilder {
	return &PowerSupplyScheduleMessageBuilder{
		shutdowns: shutdowns,
		now:       now,
	}
}

func (mb *PowerSupplyScheduleMessageBuilder) WithNextDay(nextDayShutdowns dal.Shutdowns) *PowerSupplyScheduleMessageBuilder {
	mb.nextDayTable = &nextDayShutdowns
	return mb
}

// Build generates a notification message for a subscription
// Returns PowerSupplyMessage with message and hash updates, or empty result if no changes
// If builder has next day data, tomorrowState must be provided
func (mb *PowerSupplyScheduleMessageBuilder) Build(sub dal.Subscription, todayState, tomorrowState dal.NotificationState) (PowerSupplyMessage, error) {
	result := PowerSupplyMessage{
		TodayUpdatedGroups:    make(map[string]string),
		TomorrowUpdatedGroups: make(map[string]string),
	}

	// Collect and sort group numbers to ensure deterministic order
	groupNums := make([]string, 0, len(sub.Groups))
	for groupNum := range sub.Groups {
		groupNums = append(groupNums, groupNum)
	}

	// Sort numerically (e.g., "1", "2", "11" -> 1, 2, 11 not "1", "11", "2")
	sort.Slice(groupNums, func(i, j int) bool {
		numI, _ := strconv.Atoi(groupNums[i])
		numJ, _ := strconv.Atoi(groupNums[j])
		return numI < numJ
	})

	// Collect date schedules (today + tomorrow max)
	const maxDates = 2
	dateSchedules := make([]DateSchedule, 0, maxDates)

	// Process today's schedule
	todaySchedule := mb.processDateSchedule(mb.shutdowns, todayState, groupNums, mb.now)
	if len(todaySchedule.UpdatedGroups) > 0 {
		dateSchedules = append(dateSchedules, DateSchedule{
			Date:   mb.shutdowns.Date,
			Groups: todaySchedule.Groups,
		})
		result.TodayUpdatedGroups = todaySchedule.UpdatedGroups
	}

	// Process tomorrow's schedule if available
	if mb.nextDayTable != nil {
		// For tomorrow, show all periods (start from midnight)
		tomorrowTime := time.Date(2000, 1, 1, 0, 0, 0, 0, mb.now.Location()) // Use arbitrary date with 00:00
		tomorrowSchedule := mb.processDateSchedule(*mb.nextDayTable, tomorrowState, groupNums, tomorrowTime)
		if len(tomorrowSchedule.UpdatedGroups) > 0 {
			dateSchedules = append(dateSchedules, DateSchedule{
				Date:   mb.nextDayTable.Date,
				Groups: tomorrowSchedule.Groups,
			})
			result.TomorrowUpdatedGroups = tomorrowSchedule.UpdatedGroups
		}
	}

	if len(dateSchedules) == 0 {
		return result, nil
	}

	// Render multi-date message
	msg, err := renderMultiDateMessage(dateSchedules)
	if err != nil {
		return PowerSupplyMessage{}, fmt.Errorf("render message: %w", err)
	}

	result.Text = msg

	return result, nil
}

// dateScheduleResult holds the result of processing a single date's schedule
type dateScheduleResult struct {
	Groups        []GroupSchedule
	UpdatedGroups map[string]string
}

// processDateSchedule processes shutdown schedule for a single date
func (mb *PowerSupplyScheduleMessageBuilder) processDateSchedule(
	shutdowns dal.Shutdowns,
	notifState dal.NotificationState,
	groupNums []string,
	filterTime time.Time,
) dateScheduleResult {
	result := dateScheduleResult{
		Groups:        make([]GroupSchedule, 0),
		UpdatedGroups: make(map[string]string),
	}

	for _, groupNum := range groupNums {
		currentHash := notifState.Hashes[groupNum]

		group, ok := shutdowns.Groups[groupNum]
		if !ok {
			continue
		}

		newHash := shutdownGroupHash(group)
		if currentHash == newHash {
			continue
		}

		joinedPeriods, joinedStatuses := join(shutdowns.Periods, group.Items)
		cutPeriods, cutStatuses := cutByTime(filterTime, joinedPeriods, joinedStatuses)

		groupSchedule := buildGroupSchedule(groupNum, cutPeriods, cutStatuses)

		result.Groups = append(result.Groups, groupSchedule)
		result.UpdatedGroups[groupNum] = newHash
	}

	return result
}

// shutdownGroupHash generates a hash for a shutdown group
func shutdownGroupHash(g dal.ShutdownGroup) string {
	var buf bytes.Buffer
	for _, i := range g.Items {
		buf.WriteString(string(i))
	}
	return buf.String()
}

// join merges consecutive periods with the same status
func join(periods []dal.Period, statuses []dal.Status) ([]dal.Period, []dal.Status) {
	if len(periods) == 0 {
		return []dal.Period{}, []dal.Status{}
	}

	groupedPeriod := make([]dal.Period, 0)
	groupedStatus := make([]dal.Status, 0)

	currentFrom := periods[0].From
	currentTo := periods[0].To
	currentStatus := statuses[0]
	for i := 1; i < len(periods); i++ {
		if statuses[i] == currentStatus {
			currentTo = periods[i].To
			continue
		}
		groupedPeriod = append(groupedPeriod, dal.Period{From: currentFrom, To: currentTo})
		groupedStatus = append(groupedStatus, currentStatus)
		currentFrom = periods[i].From
		currentTo = periods[i].To
		currentStatus = statuses[i]
	}
	groupedPeriod = append(groupedPeriod, dal.Period{From: currentFrom, To: currentTo})
	groupedStatus = append(groupedStatus, currentStatus)

	return groupedPeriod, groupedStatus
}

// cutByTime filters out past periods based on the provided time
func cutByTime(now time.Time, periods []dal.Period, items []dal.Status) ([]dal.Period, []dal.Status) {
	currentTime := now.Format("15:04")

	cutPeriods := make([]dal.Period, 0)
	cutItems := make([]dal.Status, 0)
	for i, p := range periods {
		if periods[i].To > currentTime {
			cutPeriods = append(cutPeriods, p)
			cutItems = append(cutItems, items[i])
		}
	}

	return cutPeriods, cutItems
}

// StatusLine represents a single status type with its periods
type StatusLine struct {
	Emoji   string
	Label   string
	Periods []dal.Period
}

// GroupSchedule represents schedule for a single group
type GroupSchedule struct {
	GroupNum    string
	StatusLines []StatusLine
}

// DateSchedule represents schedule for a single date
type DateSchedule struct {
	Date   string
	Groups []GroupSchedule
}

// NotificationMessage represents the complete notification structure
type NotificationMessage struct {
	Dates []DateSchedule
}

// buildGroupSchedule creates a GroupSchedule from periods and statuses
func buildGroupSchedule(num string, periods []dal.Period, statuses []dal.Status) GroupSchedule {
	grouped := make(map[dal.Status][]dal.Period)

	for i, p := range periods {
		grouped[statuses[i]] = append(grouped[statuses[i]], p)
	}

	// IMPORTANT: If you change these emojis or labels, update CLAUDE.md and TEMPLATES.md
	statusLines := []StatusLine{
		{Emoji: "🟢", Label: "Заживлено", Periods: grouped[dal.ON]},
		{Emoji: "🟡", Label: "Можливо заживлено", Periods: grouped[dal.MAYBE]},
		{Emoji: "🔴", Label: "Відключено", Periods: grouped[dal.OFF]},
	}

	return GroupSchedule{
		GroupNum:    num,
		StatusLines: statusLines,
	}
}

// renderMultiDateMessage renders a message for multiple dates
func renderMultiDateMessage(dates []DateSchedule) (string, error) {
	msg := NotificationMessage{
		Dates: dates,
	}

	var buf bytes.Buffer
	err := messageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
