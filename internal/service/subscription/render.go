package subscription

import (
	"bytes"
	"text/template"

	"github.com/Roma7-7-7/sso-notifier/models"
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
  🟡 Можливо заживлено: {{range .Maybe}} {{.From}} - {{.To}}; {{end}}
  🔴 Відключено: {{range .Off}} {{.From}} - {{.To}}; {{end}}
`))

type groupMessage struct {
	GroupNum string
	On       []models.Period
	Off      []models.Period
	Maybe    []models.Period
}

func renderMessage(date string, msgs []string) (string, error) {
	var buf bytes.Buffer
	err := messageTemplate.Execute(&buf, message{Date: date, Msgs: msgs})
	return buf.String(), err
}

func renderGroup(num string, periods []models.Period, statuses []models.Status) (string, error) {
	grouped := make(map[models.Status][]models.Period)

	for i := 0; i < len(periods); i++ {
		grouped[statuses[i]] = append(grouped[statuses[i]], periods[i])
	}

	msg := groupMessage{
		GroupNum: num,
		On:       grouped[models.ON],
		Off:      grouped[models.OFF],
		Maybe:    grouped[models.MAYBE],
	}

	var buf bytes.Buffer
	err := groupMessageTemplate.Execute(&buf, msg)
	return buf.String(), err
}
