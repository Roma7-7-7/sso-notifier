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

	for i := 0; i < len(periods); i++ {
		grouped[statuses[i]] = append(grouped[statuses[i]], periods[i])
	}

	msg := groupMessage{
		GroupNum: num,
		On:       grouped[ON],
		Off:      grouped[OFF],
	}

	var buf bytes.Buffer
	err := groupMessageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
