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

// MessageBuilder builds notification messages for subscribed users
type MessageBuilder struct {
	date      string
	shutdowns dal.Shutdowns
	now       time.Time
}

// NewMessageBuilder creates a new message builder for a specific date and shutdowns data
func NewMessageBuilder(date string, shutdowns dal.Shutdowns, now time.Time) *MessageBuilder {
	return &MessageBuilder{
		date:      date,
		shutdowns: shutdowns,
		now:       now,
	}
}

// Message contains the built message and updated subscription
type Message struct {
	Text          string
	UpdatedGroups map[string]string // groupNum -> newHash
}

// Build generates a notification message for a subscription
// Returns Message with message and hash updates, or empty result if no changes
func (mb *MessageBuilder) Build(sub dal.Subscription, notifState dal.NotificationState) (Message, error) {
	result := Message{
		UpdatedGroups: make(map[string]string),
	}

	groupSchedules := make([]GroupSchedule, 0)

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

	for _, groupNum := range groupNums {
		// Get current hash from notification state
		currentHash := notifState.Hashes[groupNum]

		group, ok := mb.shutdowns.Groups[groupNum]
		if !ok {
			continue
		}

		// Hack to make sure updates for new day will be sent even if there is no changes in schedule
		newHash := shutdownGroupHash(group, fmt.Sprintf("%s:", mb.date))
		if currentHash == newHash {
			continue
		}

		// Process group shutdown periods
		joinedPeriods, joinedStatuses := join(mb.shutdowns.Periods, group.Items)
		cutPeriods, cutStatuses := cutByTime(mb.now, joinedPeriods, joinedStatuses)

		// Build group schedule
		groupSchedule := buildGroupSchedule(groupNum, cutPeriods, cutStatuses)

		groupSchedules = append(groupSchedules, groupSchedule)
		result.UpdatedGroups[groupNum] = newHash
	}

	if len(groupSchedules) == 0 {
		return result, nil
	}

	// Render final message using template
	msg, err := renderMessage(mb.date, groupSchedules)
	if err != nil {
		return Message{}, fmt.Errorf("render message: %w", err)
	}

	result.Text = msg

	return result, nil
}

// shutdownGroupHash generates a hash for a shutdown group
func shutdownGroupHash(g dal.ShutdownGroup, prefix string) string {
	var buf bytes.Buffer

	buf.WriteString(prefix)
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

// messageTemplate is the main notification template
// IMPORTANT: If you change this template or the rendering logic below, you must also update:
// - internal/service/TEMPLATES.md - Update examples and documentation
// - CLAUDE.md - Update message format examples in "Message Templates" section
//
//nolint:gochecknoglobals // it's template
var messageTemplate = template.Must(template.New("message").Parse(`Графік стабілізаційних відключень:
{{range .Dates}}
📅 {{.Date}}:
{{range .Groups}}Група {{.GroupNum}}:
{{range .StatusLines}}{{if .Periods}}  {{.Emoji}} {{.Label}}:{{range .Periods}} {{.From}} - {{.To}};{{end}}
{{end}}{{end}}
{{end}}{{end}}`))

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

func renderMessage(date string, groups []GroupSchedule) (string, error) {
	msg := NotificationMessage{
		Dates: []DateSchedule{
			{
				Date:   date,
				Groups: groups,
			},
		},
	}

	var buf bytes.Buffer
	err := messageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
