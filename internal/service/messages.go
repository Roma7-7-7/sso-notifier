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
		Images                []ScheduleImage   // filled only when the builder is created WithImages
	}

	MessageBuilder struct {
		template         *template.Template
		withPeriodRanges bool // If true, then linear template shows "🟢 11:30 - 13:00" instead of "🟢 11:30"
		withImages       bool // If true, Build also returns per group full-day schedules for image rendering

		shutdowns    dal.Shutdowns
		nextDayTable *dal.Shutdowns // Optional next day shutdowns
		now          time.Time
	}
)

func newMessageBuilder(template *template.Template, shutdowns dal.Shutdowns, now time.Time) *MessageBuilder {
	return &MessageBuilder{
		template:  template,
		shutdowns: shutdowns,
		now:       now,
	}
}

func (mb *MessageBuilder) WithNextDay(shutdowns dal.Shutdowns) *MessageBuilder {
	mb.nextDayTable = &shutdowns
	return mb
}

// WithImages makes Build return ScheduleImage for every updated group. Text is still built, to be used as a fallback.
func (mb *MessageBuilder) WithImages() *MessageBuilder {
	mb.withImages = true
	return mb
}

type dateSchedule struct {
	Date   string
	Groups []groupSchedule
}

type renderable struct {
	Dates []dateSchedule
}

func (mb *MessageBuilder) Build(sub dal.Subscription, todayState, tomorrowState dal.NotificationState) (PowerSupplyScheduleMessage, error) {
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
	dateSchedules := make([]dateSchedule, 0, maxDates)

	todaySchedule := mb.processDateSchedule(mb.shutdowns, todayState, groupNums, mb.now)
	if len(todaySchedule.UpdatedGroups) > 0 {
		dateSchedules = append(dateSchedules, dateSchedule{
			Date:   mb.shutdowns.Date,
			Groups: todaySchedule.Groups,
		})
		result.TodayUpdatedGroups = todaySchedule.UpdatedGroups
		result.Images = append(result.Images, todaySchedule.Images...)
	}

	if mb.nextDayTable != nil {
		tomorrowTime := time.Date(2000, 1, 1, 0, 0, 0, 0, mb.now.Location())
		tomorrowSchedule := mb.processDateSchedule(*mb.nextDayTable, tomorrowState, groupNums, tomorrowTime)
		if len(tomorrowSchedule.UpdatedGroups) > 0 {
			dateSchedules = append(dateSchedules, dateSchedule{
				Date:   mb.nextDayTable.Date,
				Groups: tomorrowSchedule.Groups,
			})
			result.TomorrowUpdatedGroups = tomorrowSchedule.UpdatedGroups
			result.Images = append(result.Images, tomorrowSchedule.Images...)
		}
	}

	if len(dateSchedules) == 0 {
		return result, nil
	}

	buff := &bytes.Buffer{}
	err := mb.template.Execute(buff, renderable{
		Dates: dateSchedules,
	})
	if err != nil {
		return PowerSupplyScheduleMessage{}, fmt.Errorf("execute template: %w", err)
	}

	result.Text = buff.String()
	return result, nil
}

type (
	statusLine struct {
		Emoji   string
		Label   string
		Periods []dal.Period
	}

	groupSchedule struct {
		GroupNum       string
		StatusLines    []statusLine
		LinearTimeline string
	}

	processDateScheduleResul struct {
		Groups        []groupSchedule
		UpdatedGroups map[string]string
		Images        []ScheduleImage
	}
)

func (mb *MessageBuilder) processDateSchedule(
	shutdowns dal.Shutdowns,
	notifState dal.NotificationState,
	groupNums []string,
	filterTime time.Time,
) processDateScheduleResul {
	result := processDateScheduleResul{
		Groups:        make([]groupSchedule, 0, len(groupNums)),
		UpdatedGroups: make(map[string]string, len(groupNums)),
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

		processed := buildGroupedSchedule(groupNum, cutPeriods, cutStatuses)
		processed.LinearTimeline = buildLinearSchedule(cutPeriods, cutStatuses, mb.withPeriodRanges)
		result.Groups = append(result.Groups, processed)
		result.UpdatedGroups[groupNum] = newHash

		if mb.withImages {
			// images show the whole day, unlike text which is cut to the upcoming periods
			result.Images = append(result.Images, ScheduleImage{
				Date:     shutdowns.Date,
				GroupNum: groupNum,
				Periods:  joinedPeriods,
				Statuses: joinedStatuses,
			})
		}
	}

	return result
}

func buildGroupedSchedule(num string, periods []dal.Period, statuses []dal.Status) groupSchedule {
	grouped := make(map[dal.Status][]dal.Period)

	for i, p := range periods {
		grouped[statuses[i]] = append(grouped[statuses[i]], p)
	}

	// IMPORTANT: If you change these emojis or labels, update CLAUDE.md and TEMPLATES.md
	statusLines := []statusLine{
		{Emoji: "🟢", Label: "Заживлено", Periods: grouped[dal.ON]},
		{Emoji: "🟡", Label: "Можливо заживлено", Periods: grouped[dal.MAYBE]},
		{Emoji: "🔴", Label: "Відключено", Periods: grouped[dal.OFF]},
	}

	return groupSchedule{
		GroupNum:    num,
		StatusLines: statusLines,
	}
}

func buildLinearSchedule(periods []dal.Period, statuses []dal.Status, withEndPeriod bool) string {
	if len(periods) == 0 {
		return ""
	}

	var parts []string
	for i, period := range periods {
		emoji := getEmojiForStatus(statuses[i])
		if withEndPeriod {
			parts = append(parts, fmt.Sprintf("%s %s - %s", emoji, period.From, period.To))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s", emoji, period.From))
		}
	}

	return strings.Join(parts, " | ")
}

//nolint:gochecknoglobals // template must be initialized once
var groupedTemplate = template.Must(template.New("message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}:
{{range .StatusLines}}{{if .Periods}}  {{.Emoji}} {{.Label}}:{{range .Periods}} {{.From}} - {{.To}};{{end}}
{{end}}{{end}}
{{end}}{{end}}`))

func NewGroupedMessageBuilder(shutdowns dal.Shutdowns, now time.Time) *MessageBuilder {
	return newMessageBuilder(groupedTemplate, shutdowns, now)
}

//nolint:gochecknoglobals // template must be initialized once
var linearTemplate = template.Must(template.New("linear_message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}: 
{{.LinearTimeline}}

{{end}}{{end}}`))

func NewLinearMessageBuilder(shutdowns dal.Shutdowns, withPeriodRange bool, now time.Time) *MessageBuilder {
	res := newMessageBuilder(linearTemplate, shutdowns, now)
	res.withPeriodRanges = withPeriodRange
	return res
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

//nolint:gochecknoglobals // template must be initialized once
var powerSupplyChangeTemplate = template.Must(
	template.New("upcoming").Funcs(template.FuncMap{
		"joinGroups": func(groups []string) string {
			return strings.Join(groups, ", ")
		},
	}).Parse(`⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.
{{range .}}
{{if eq (len .Groups) 1}}Група {{index .Groups 0}}:{{else}}Групи {{joinGroups .Groups}}:{{end}}
{{.Emoji}} {{.Label}} об {{.StartTime}}
{{end}}`))

func NewPowerSupplyChangeMessageBuilder() *PowerSupplyChangeMessageBuilder {
	return &PowerSupplyChangeMessageBuilder{
		template: powerSupplyChangeTemplate,
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
