package service

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

// UpcomingAlert represents a single upcoming status change alert
type UpcomingAlert struct {
	Status    dal.Status
	StartTime string
	Groups    []string // Group numbers (e.g., ["5", "7"])
	Emoji     string   // Status emoji (🟢/🟡/🔴)
	Label     string   // Status label in Ukrainian
}

// UpcomingMessage represents the complete upcoming notification structure
type UpcomingMessage struct {
	IsRestoration bool            // true if any alert is for ON status
	Alerts        []UpcomingAlert // Grouped alerts by status+time
}

// upcomingMessageTemplate is the template for 10-minute advance notifications
// IMPORTANT: If you change this template or the rendering logic below, you must also update:
// - internal/service/TEMPLATES.md - Update "Upcoming Notification Template" section
// - CLAUDE.md - Update message format examples in "Alerts Service" section
//
//nolint:gochecknoglobals // it's template
var upcomingMessageTemplate = template.Must(
	template.New("upcoming").Funcs(template.FuncMap{
		"joinGroups": func(groups []string) string {
			return strings.Join(groups, ", ")
		},
	}).Parse(`⚠️ Увага! Через 10 хвилин:
{{range .Alerts}}
{{if eq (len .Groups) 1}}Група {{index .Groups 0}}:{{else}}Групи {{joinGroups .Groups}}:{{end}}
{{.Emoji}} {{.Label}} об {{.StartTime}}
{{end}}`))

// renderUpcomingMessage renders the notification message for upcoming events
func renderUpcomingMessage(alerts []PendingAlert) string {
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

	// Convert to UpcomingAlert structs
	upcomingAlerts := make([]UpcomingAlert, 0, len(grouped))

	for key, groups := range grouped {
		// Sort groups numerically
		sort.Slice(groups, func(i, j int) bool {
			numI, _ := strconv.Atoi(groups[i])
			numJ, _ := strconv.Atoi(groups[j])
			return numI < numJ
		})

		upcomingAlerts = append(upcomingAlerts, UpcomingAlert{
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

	msg := UpcomingMessage{
		Alerts: upcomingAlerts,
	}

	var buf bytes.Buffer
	if err := upcomingMessageTemplate.Execute(&buf, msg); err != nil {
		// Fallback to simple message on template error
		return "⚠️ Увага! Через 10 хвилин змінюється статус електроенергії"
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
		return "Відновлення електроенергії"
	case dal.OFF:
		return "Відключення електроенергії"
	case dal.MAYBE:
		return "Можливе відключення/відновлення електроенергії"
	default:
		return "Невідомий статус"
	}
}
