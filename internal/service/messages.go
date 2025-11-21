package service

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

type (
	PowerSupplyScheduleMessage struct {
		Text                  string
		TodayUpdatedGroups    map[string]string // groupNum -> newHash for today
		TomorrowUpdatedGroups map[string]string // groupNum -> newHash for tomorrow (if applicable)
	}

	PowerSupplyScheduleMessageBuilder struct {
		shutdowns    dal.Shutdowns
		nextDayTable *dal.Shutdowns // Optional next day shutdowns
		now          time.Time

		template *template.Template
	}
)

func NewPowerSupplyScheduleMessageBuilder(shutdowns dal.Shutdowns, now time.Time) *PowerSupplyScheduleMessageBuilder {
	// powerSupplyScheduleMessageTemplate is the main notification template
	// IMPORTANT: If you change this template or the rendering logic below, you must also update:
	// - internal/service/TEMPLATES.md - Update examples and documentation
	// - CLAUDE.md - Update message format examples in "PowerSupplyScheduleMessage Templates" section
	templ := template.Must(template.New("message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}:
{{range .StatusLines}}{{if .Periods}}  {{.Emoji}} {{.Label}}:{{range .Periods}} {{.From}} - {{.To}};{{end}}
{{end}}{{end}}
{{end}}{{end}}`))
	return &PowerSupplyScheduleMessageBuilder{
		shutdowns: shutdowns,
		now:       now,

		template: templ,
	}
}

func (mb *PowerSupplyScheduleMessageBuilder) WithNextDay(nextDayShutdowns dal.Shutdowns) *PowerSupplyScheduleMessageBuilder {
	mb.nextDayTable = &nextDayShutdowns
	return mb
}

// Build generates a notification message for a subscription
// Returns PowerSupplyScheduleMessage with message and hash updates, or empty result if no changes
// If builder has next day data, tomorrowState must be provided
func (mb *PowerSupplyScheduleMessageBuilder) Build(sub dal.Subscription, todayState, tomorrowState dal.NotificationState) (PowerSupplyScheduleMessage, error) {
	result := PowerSupplyScheduleMessage{
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
	msg, err := mb.renderMultiDateMessage(dateSchedules)
	if err != nil {
		return PowerSupplyScheduleMessage{}, fmt.Errorf("render message: %w", err)
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

// renderMultiDateMessage renders a message for multiple dates
func (mb *PowerSupplyScheduleMessageBuilder) renderMultiDateMessage(dates []DateSchedule) (string, error) {
	msg := NotificationMessage{
		Dates: dates,
	}

	var buf bytes.Buffer
	err := mb.template.Execute(&buf, msg)
	return buf.String(), err
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

type (
	PowerSupplyChangeMessage struct {
		Status    dal.Status
		StartTime string
		Groups    []string // Group numbers (e.g., ["5", "7"])
		Emoji     string   // Status emoji (🟢/🟡/🔴)
		Label     string   // Status label in Ukrainian
	}

	PowerSupplyChangeMessageBuilder struct {
		template *template.Template
	}
)

func NewPowerSupplyChangeMessageBuilder() *PowerSupplyChangeMessageBuilder {
	// powerSupplyChangeMessageTemplate is the template for 10-minute advance notifications
	// IMPORTANT: If you change this template or the rendering logic below, you must also update:
	// - internal/service/TEMPLATES.md - Update "Upcoming Notification Template" section
	// - CLAUDE.md - Update message format examples in "Alerts Service" section
	templ := template.Must(
		template.New("upcoming").Funcs(template.FuncMap{
			"joinGroups": func(groups []string) string {
				return strings.Join(groups, ", ")
			},
		}).Parse(`⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.
{{range .}}
{{if eq (len .Groups) 1}}Група {{index .Groups 0}}:{{else}}Групи {{joinGroups .Groups}}:{{end}}
{{.Emoji}} {{.Label}} об {{.StartTime}}
{{end}}`))

	return &PowerSupplyChangeMessageBuilder{
		template: templ,
	}
}

func (b *PowerSupplyChangeMessageBuilder) Build(alerts []Alert) string {
	if len(alerts) == 0 {
		return ""
	}

	type groupKey struct {
		Status    dal.Status
		StartTime string
	}

	// Group alerts by status and start time
	grouped := make(map[groupKey][]string)
	for _, a := range alerts {
		key := groupKey{Status: a.Status, StartTime: a.StartTime}
		grouped[key] = append(grouped[key], a.GroupNum)
	}

	// Convert to PowerSupplyChangeMessage structs
	upcomingAlerts := make([]PowerSupplyChangeMessage, 0, len(grouped))

	for key, groups := range grouped {
		// Sort groups numerically
		sort.Slice(groups, func(i, j int) bool {
			numI, _ := strconv.Atoi(groups[i])
			numJ, _ := strconv.Atoi(groups[j])
			return numI < numJ
		})

		upcomingAlerts = append(upcomingAlerts, PowerSupplyChangeMessage{
			Status:    key.Status,
			StartTime: key.StartTime,
			Groups:    groups,
			Emoji:     getEmojiForStatus(key.Status),
			Label:     getLabelForStatus(key.Status),
		})
	}

	// Sort alerts by start time, then by minimum group number, then by status priority
	sort.Slice(upcomingAlerts, func(i, j int) bool {
		if upcomingAlerts[i].StartTime != upcomingAlerts[j].StartTime {
			return upcomingAlerts[i].StartTime < upcomingAlerts[j].StartTime
		}

		// Get minimum group number for each alert (groups are already sorted)
		minGroupI, _ := strconv.Atoi(upcomingAlerts[i].Groups[0])
		minGroupJ, _ := strconv.Atoi(upcomingAlerts[j].Groups[0])

		if minGroupI != minGroupJ {
			return minGroupI < minGroupJ
		}

		return statusPriority(upcomingAlerts[i].Status) < statusPriority(upcomingAlerts[j].Status)
	})

	var buf bytes.Buffer
	if err := b.template.Execute(&buf, upcomingAlerts); err != nil {
		// Fallback to simple message on template error
		return "⚠️ Увага! Незабаром зміниться електропостачання"
	}

	return strings.TrimSpace(buf.String())
}

// statusPriority returns sort priority for status (lower = higher priority)
func statusPriority(status dal.Status) int {
	const (
		priorityOff     = 0
		priorityMaybe   = 1
		priorityOn      = 2
		priorityDefault = 3
	)
	switch status {
	case dal.OFF:
		return priorityOff
	case dal.MAYBE:
		return priorityMaybe
	case dal.ON:
		return priorityOn
	default:
		return priorityDefault
	}
}

// getEmojiForStatus returns the emoji for a status
func getEmojiForStatus(status dal.Status) string {
	switch status {
	case dal.ON:
		return "🟢"
	case dal.OFF:
		return "🔴"
	case dal.MAYBE:
		return "🟡"
	default:
		return "⚪"
	}
}

// getLabelForStatus returns the Ukrainian label for a status
func getLabelForStatus(status dal.Status) string {
	switch status {
	case dal.ON:
		return "Відновлення електропостачання"
	case dal.OFF:
		return "Відключення електропостачання"
	case dal.MAYBE:
		return "Можливе відключення/відновлення електропостачання"
	default:
		return "Невідомий статус"
	}
}

// PowerSupplyScheduleLinearMessageBuilder builds linear timeline messages
type PowerSupplyScheduleLinearMessageBuilder struct {
	shutdowns    dal.Shutdowns
	nextDayTable *dal.Shutdowns
	now          time.Time
	template     *template.Template
}

// LinearGroupSchedule represents a linear timeline for a single group
type LinearGroupSchedule struct {
	GroupNum string
	Timeline string
}

// LinearDateSchedule represents a linear schedule for a single date
type LinearDateSchedule struct {
	Date   string
	Groups []LinearGroupSchedule
}

// LinearNotificationMessage represents the complete linear notification structure
type LinearNotificationMessage struct {
	Dates []LinearDateSchedule
}

func NewPowerSupplyScheduleLinearMessageBuilder(shutdowns dal.Shutdowns, now time.Time) *PowerSupplyScheduleLinearMessageBuilder {
	templ := template.Must(template.New("linear_message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}: 
{{.Timeline}}

{{end}}{{end}}`))

	return &PowerSupplyScheduleLinearMessageBuilder{
		shutdowns: shutdowns,
		now:       now,
		template:  templ,
	}
}

func (mb *PowerSupplyScheduleLinearMessageBuilder) WithNextDay(nextDayShutdowns dal.Shutdowns) *PowerSupplyScheduleLinearMessageBuilder {
	mb.nextDayTable = &nextDayShutdowns
	return mb
}

func (mb *PowerSupplyScheduleLinearMessageBuilder) Build(sub dal.Subscription, todayState, tomorrowState dal.NotificationState) (PowerSupplyScheduleMessage, error) {
	result := PowerSupplyScheduleMessage{
		TodayUpdatedGroups:    make(map[string]string),
		TomorrowUpdatedGroups: make(map[string]string),
	}

	groupNums := make([]string, 0, len(sub.Groups))
	for groupNum := range sub.Groups {
		groupNums = append(groupNums, groupNum)
	}

	sort.Slice(groupNums, func(i, j int) bool {
		numI, _ := strconv.Atoi(groupNums[i])
		numJ, _ := strconv.Atoi(groupNums[j])
		return numI < numJ
	})

	const maxDates = 2
	dateSchedules := make([]LinearDateSchedule, 0, maxDates)

	todaySchedule := mb.processDateScheduleLinear(mb.shutdowns, todayState, groupNums, mb.now)
	if len(todaySchedule.UpdatedGroups) > 0 {
		dateSchedules = append(dateSchedules, LinearDateSchedule{
			Date:   mb.shutdowns.Date,
			Groups: todaySchedule.Groups,
		})
		result.TodayUpdatedGroups = todaySchedule.UpdatedGroups
	}

	if mb.nextDayTable != nil {
		tomorrowTime := time.Date(2000, 1, 1, 0, 0, 0, 0, mb.now.Location())
		tomorrowSchedule := mb.processDateScheduleLinear(*mb.nextDayTable, tomorrowState, groupNums, tomorrowTime)
		if len(tomorrowSchedule.UpdatedGroups) > 0 {
			dateSchedules = append(dateSchedules, LinearDateSchedule{
				Date:   mb.nextDayTable.Date,
				Groups: tomorrowSchedule.Groups,
			})
			result.TomorrowUpdatedGroups = tomorrowSchedule.UpdatedGroups
		}
	}

	if len(dateSchedules) == 0 {
		return result, nil
	}

	msg, err := mb.renderLinearMessage(dateSchedules)
	if err != nil {
		return PowerSupplyScheduleMessage{}, fmt.Errorf("render linear message: %w", err)
	}

	result.Text = msg
	return result, nil
}

type linearDateScheduleResult struct {
	Groups        []LinearGroupSchedule
	UpdatedGroups map[string]string
}

func (mb *PowerSupplyScheduleLinearMessageBuilder) processDateScheduleLinear(
	shutdowns dal.Shutdowns,
	notifState dal.NotificationState,
	groupNums []string,
	filterTime time.Time,
) linearDateScheduleResult {
	result := linearDateScheduleResult{
		Groups:        make([]LinearGroupSchedule, 0),
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

		timeline := buildLinearTimeline(cutPeriods, cutStatuses)

		result.Groups = append(result.Groups, LinearGroupSchedule{
			GroupNum: groupNum,
			Timeline: timeline,
		})
		result.UpdatedGroups[groupNum] = newHash
	}

	return result
}

func buildLinearTimeline(periods []dal.Period, statuses []dal.Status) string {
	if len(periods) == 0 {
		return ""
	}

	var parts []string
	for i, period := range periods {
		emoji := getEmojiForStatus(statuses[i])
		parts = append(parts, fmt.Sprintf("%s %s", emoji, period.From))
	}

	return strings.Join(parts, " | ")
}

func (mb *PowerSupplyScheduleLinearMessageBuilder) renderLinearMessage(dates []LinearDateSchedule) (string, error) {
	msg := LinearNotificationMessage{
		Dates: dates,
	}

	var buf bytes.Buffer
	err := mb.template.Execute(&buf, msg)
	return buf.String(), err
}
