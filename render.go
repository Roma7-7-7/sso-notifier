package main

import (
	"bytes"
	"text/template"
)

var messageTemplate = template.Must(template.New("message").Parse(`
Графік стабілізаційних відключень на {{.Date}}:

{{range .Msgs}} {{.}}
{{end}}
`))

type message struct {
	Date string
	Msgs []string
}

var groupMessageTemplate = template.Must(template.New("groupMessage").Parse(`Група {{.GroupNum}}:
  🟢 Заживлено:  {{range .On}} {{.From}} - {{.To}}; {{end}}
  🔴 Відключено: {{range .Off}} {{.From}} - {{.To}}; {{end}}
`))

type groupMessage struct {
	GroupNum string
	On       []Period
	Off      []Period
}

func renderMessage(date string, msgs []string) (string, error) {
	var buf bytes.Buffer
	err := messageTemplate.Execute(&buf, message{Date: date, Msgs: msgs})
	return buf.String(), err
}

func renderGroup(num string, periods []Period, statuses []Status) (string, error) {
	grouped := make(map[Status][]Period)
	currentFrom := periods[0].From
	currentTo := periods[0].To
	status := statuses[0]

	for i := 1; i < len(periods); i++ {
		if statuses[i] == status {
			currentTo = periods[i].To
			continue
		}
		if _, ok := grouped[status]; !ok {
			grouped[status] = make([]Period, 0)
		}
		grouped[status] = append(grouped[status], Period{From: currentFrom, To: currentTo})

		currentFrom = periods[i].From
		currentTo = periods[i].To
		status = statuses[i]
	}
	grouped[status] = append(grouped[status], Period{From: currentFrom, To: currentTo})

	msg := groupMessage{
		GroupNum: num,
		On:       grouped[ON],
		Off:      grouped[OFF],
	}

	var buf bytes.Buffer
	err := groupMessageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
