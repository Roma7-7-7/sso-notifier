package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

type TelegramClient interface {
	SendMessage(context.Context, string, string) error
}

type Notifications struct {
	shutdowns     ShutdownsStore
	subscriptions SubscriptionsStore
	telegram      TelegramClient

	loc *time.Location
	log *slog.Logger
	mx  *sync.Mutex
}

func NewNotifications(shutdowns ShutdownsStore, subscriptions SubscriptionsStore, telegram TelegramClient, loc *time.Location, log *slog.Logger) *Notifications {
	return &Notifications{
		shutdowns:     shutdowns,
		subscriptions: subscriptions,
		telegram:      telegram,

		loc: loc,

		log: log.With("component", "service").With("service", "notifications"),
		mx:  &sync.Mutex{},
	}
}

func (s *Notifications) NotifyShutdownUpdates(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "Notifying about shoutdown updates")

	table, ok, err := s.shutdowns.GetShutdowns(dal.TodayDate(s.loc))
	if err != nil {
		return fmt.Errorf("getting shutdowns table: %w", err)
	}
	if !ok {
		// table is not ready yet
		s.log.InfoContext(ctx, "No shoutdown updates available")
		return nil
	}
	grouped := make(map[string]dal.ShutdownGroup)
	for k, v := range table.Groups {
		grouped[k] = v
	}

	subs, err := s.subscriptions.GetAllSubscriptions()
	if err != nil {
		return fmt.Errorf("getting all subscriptions: %w", err)
	}

	for _, sub := range subs {
		s.processSubscription(ctx, sub, table, grouped)
	}

	return nil
}

func (s *Notifications) processSubscription(ctx context.Context, sub dal.Subscription, table dal.Shutdowns, grouped map[string]dal.ShutdownGroup) {
	msgs := make([]string, 0)

	chatID := sub.ChatID
	log := s.log.With("chatID", chatID)

	for groupNum, hash := range sub.Groups {
		// Hack to make sure updates for new day will be sent even if there is no changes in schedule
		newHash := shutdownGroupHash(grouped[groupNum], fmt.Sprintf("%s:", table.Date))
		if hash == newHash {
			continue
		}

		gropuedPeriod, groupedStatuses := join(table.Periods, grouped[groupNum].Items)
		cutPeriod, cutStatuses := cutByKyivTime(s.loc, gropuedPeriod, groupedStatuses)
		msg, err := renderGroup(groupNum, cutPeriod, cutStatuses)
		if err != nil {
			log.ErrorContext(ctx, "failed to render group message", "group", groupNum, "error", err)
			return
		}
		msgs = append(msgs, msg)
		sub.Groups[groupNum] = newHash
	}

	if len(msgs) == 0 {
		return
	}

	msg, err := renderMessage(table.Date, msgs)
	if err != nil {
		log.ErrorContext(ctx, "failed to render message", "error", err)
		return
	}
	if err := s.telegram.SendMessage(ctx, strconv.FormatInt(chatID, 10), msg); err != nil {
		log.ErrorContext(ctx, "failed to send message", "error", err)
		return
	}

	if err := s.subscriptions.PutSubscription(sub); err != nil {
		log.ErrorContext(ctx, "failed to update subscription", "error", err)
		return
	}
}

func shutdownGroupHash(g dal.ShutdownGroup, prefix string) string {
	var buf bytes.Buffer

	buf.WriteString(prefix)
	for _, i := range g.Items {
		buf.WriteString(string(i))
	}
	return buf.String()
}

func join(periods []dal.Period, statuses []dal.Status) ([]dal.Period, []dal.Status) {
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

func cutByKyivTime(loc *time.Location, periods []dal.Period, items []dal.Status) ([]dal.Period, []dal.Status) {
	currentKyivDateTime := time.Now().In(loc).Format("15:04")

	cutPeriods := make([]dal.Period, 0)
	cutItems := make([]dal.Status, 0)
	for i := 0; i < len(periods); i++ {
		if periods[i].To > currentKyivDateTime {
			cutPeriods = append(cutPeriods, periods[i])
			cutItems = append(cutItems, items[i])
		}
	}

	return cutPeriods, cutItems
}

var messageTemplate = template.Must(template.New("message").Parse(`
Графік стабілізаційних відключень на {{.Date}}:

{{range .Msgs}} {{.}}
{{end}}
`))

var groupMessageTemplate = template.Must(template.New("groupMessage").Parse(`Група {{.GroupNum}}:
  🟢 Заживлено:  {{range .On}} {{.From}} - {{.To}}; {{end}}
  🟡 Можливо заживлено: {{range .Maybe}} {{.From}} - {{.To}}; {{end}}
  🔴 Відключено: {{range .Off}} {{.From}} - {{.To}}; {{end}}
`))

type message struct {
	Date string
	Msgs []string
}

type groupMessage struct {
	GroupNum string
	On       []dal.Period
	Off      []dal.Period
	Maybe    []dal.Period
}

func renderMessage(date string, msgs []string) (string, error) {
	var buf bytes.Buffer
	err := messageTemplate.Execute(&buf, message{Date: date, Msgs: msgs})
	return buf.String(), err
}

func renderGroup(num string, periods []dal.Period, statuses []dal.Status) (string, error) {
	grouped := make(map[dal.Status][]dal.Period)

	for i := 0; i < len(periods); i++ {
		grouped[statuses[i]] = append(grouped[statuses[i]], periods[i])
	}

	msg := groupMessage{
		GroupNum: num,
		On:       grouped[dal.ON],
		Off:      grouped[dal.OFF],
		Maybe:    grouped[dal.MAYBE],
	}

	var buf bytes.Buffer
	err := groupMessageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
