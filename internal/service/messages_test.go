package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
)

func TestMessageBuilder_Build(t *testing.T) {
	kyiv, _ := time.LoadLocation("Europe/Kyiv")

	// Helper to create periods
	periods := func(times ...string) []dal.Period {
		var result []dal.Period
		for i := 0; i < len(times); i += 2 {
			result = append(result, dal.Period{From: times[i], To: times[i+1]})
		}
		return result
	}

	// Helper to create statuses
	statuses := func(s ...dal.Status) []dal.Status {
		return s
	}

	type fields struct {
		date      string
		shutdowns dal.Shutdowns
		now       time.Time
	}
	type args struct {
		sub dal.Subscription
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    service.Message
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no changes - hash matches",
			fields: fields{
				date: "20 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "20 жовтня",
					Periods: periods("00:00", "12:00", "12:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"1": {Number: 1, Items: statuses(dal.OFF, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 20, 10, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 123,
					Groups: map[string]string{
						"1": "20 жовтня:NY", // hash matches - no notification
					},
				},
			},
			want: service.Message{
				Text:          "",
				UpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "single group - all periods in future",
			fields: fields{
				date: "20 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "20 жовтня",
					Periods: periods("12:00", "18:00", "18:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"5": {Number: 5, Items: statuses(dal.ON, dal.OFF)},
					},
				},
				now: time.Date(2024, 10, 20, 10, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 123,
					Groups: map[string]string{
						"5": "", // empty hash triggers notification
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 20 жовтня:
Група 5:
  🟢 Заживлено: 12:00 - 18:00;
  🔴 Відключено: 18:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"5": "20 жовтня:YN",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "single group - cut past periods",
			fields: fields{
				date: "20 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "20 жовтня",
					Periods: periods("00:00", "12:00", "12:00", "18:00", "18:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"3": {Number: 3, Items: statuses(dal.OFF, dal.ON, dal.OFF)},
					},
				},
				now: time.Date(2024, 10, 20, 14, 30, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 456,
					Groups: map[string]string{
						"3": "old_hash",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 20 жовтня:
Група 3:
  🟢 Заживлено: 12:00 - 18:00;
  🔴 Відключено: 18:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"3": "20 жовтня:NYN",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple groups - different statuses",
			fields: fields{
				date: "21 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "21 жовтня",
					Periods: periods("00:00", "08:00", "08:00", "16:00", "16:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"1": {Number: 1, Items: statuses(dal.ON, dal.OFF, dal.ON)},
						"2": {Number: 2, Items: statuses(dal.OFF, dal.MAYBE, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 21, 6, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 789,
					Groups: map[string]string{
						"1": "old",
						"2": "old",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 21 жовтня:
Група 1:
  🟢 Заживлено: 00:00 - 08:00; 16:00 - 24:00;
  🔴 Відключено: 08:00 - 16:00;

Група 2:
  🟢 Заживлено: 16:00 - 24:00;
  🟡 Можливо заживлено: 08:00 - 16:00;
  🔴 Відключено: 00:00 - 08:00;

`,
				UpdatedGroups: map[string]string{
					"1": "21 жовтня:YNY",
					"2": "21 жовтня:NMY",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "join consecutive periods with same status",
			fields: fields{
				date: "22 жовтня",
				shutdowns: dal.Shutdowns{
					Date: "22 жовтня",
					Periods: periods(
						"00:00", "00:30",
						"00:30", "01:00",
						"01:00", "01:30",
						"01:30", "02:00",
						"02:00", "03:00",
					),
					Groups: map[string]dal.ShutdownGroup{
						"7": {Number: 7, Items: statuses(dal.OFF, dal.OFF, dal.OFF, dal.ON, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 22, 0, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 111,
					Groups: map[string]string{
						"7": "",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 22 жовтня:
Група 7:
  🟢 Заживлено: 01:30 - 03:00;
  🔴 Відключено: 00:00 - 01:30;

`,
				UpdatedGroups: map[string]string{
					"7": "22 жовтня:NNNYY",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "all power on",
			fields: fields{
				date: "23 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "23 жовтня",
					Periods: periods("00:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"9": {Number: 9, Items: statuses(dal.ON)},
					},
				},
				now: time.Date(2024, 10, 23, 12, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 222,
					Groups: map[string]string{
						"9": "prev",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 23 жовтня:
Група 9:
  🟢 Заживлено: 00:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"9": "23 жовтня:Y",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "all power off",
			fields: fields{
				date: "24 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "24 жовтня",
					Periods: periods("00:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"10": {Number: 10, Items: statuses(dal.OFF)},
					},
				},
				now: time.Date(2024, 10, 24, 8, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 333,
					Groups: map[string]string{
						"10": "",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 24 жовтня:
Група 10:
  🔴 Відключено: 00:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"10": "24 жовтня:N",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "maybe status mixed with on/off",
			fields: fields{
				date: "25 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "25 жовтня",
					Periods: periods("00:00", "08:00", "08:00", "16:00", "16:00", "20:00", "20:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"4": {Number: 4, Items: statuses(dal.ON, dal.MAYBE, dal.OFF, dal.MAYBE)},
					},
				},
				now: time.Date(2024, 10, 25, 10, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 444,
					Groups: map[string]string{
						"4": "hash",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 25 жовтня:
Група 4:
  🟡 Можливо заживлено: 08:00 - 16:00; 20:00 - 24:00;
  🔴 Відключено: 16:00 - 20:00;

`,
				UpdatedGroups: map[string]string{
					"4": "25 жовтня:YMNM",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "cut all periods - late in day",
			fields: fields{
				date: "26 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "26 жовтня",
					Periods: periods("00:00", "12:00", "12:00", "18:00"),
					Groups: map[string]dal.ShutdownGroup{
						"6": {Number: 6, Items: statuses(dal.OFF, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 26, 23, 30, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 555,
					Groups: map[string]string{
						"6": "old",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 26 жовтня:
Група 6:

`,
				UpdatedGroups: map[string]string{
					"6": "26 жовтня:NY",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "edge case - exactly at period boundary",
			fields: fields{
				date: "27 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "27 жовтня",
					Periods: periods("00:00", "12:00", "12:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"8": {Number: 8, Items: statuses(dal.OFF, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 27, 12, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 666,
					Groups: map[string]string{
						"8": "",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 27 жовтня:
Група 8:
  🟢 Заживлено: 12:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"8": "27 жовтня:NY",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "subscription for non-existent group",
			fields: fields{
				date: "28 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "28 жовтня",
					Periods: periods("00:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"1": {Number: 1, Items: statuses(dal.ON)},
					},
				},
				now: time.Date(2024, 10, 28, 10, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 777,
					Groups: map[string]string{
						"99": "hash", // group doesn't exist
					},
				},
			},
			want: service.Message{
				Text:          "",
				UpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple groups - some changed, some not",
			fields: fields{
				date: "29 жовтня",
				shutdowns: dal.Shutdowns{
					Date:    "29 жовтня",
					Periods: periods("00:00", "12:00", "12:00", "24:00"),
					Groups: map[string]dal.ShutdownGroup{
						"1": {Number: 1, Items: statuses(dal.ON, dal.OFF)},
						"2": {Number: 2, Items: statuses(dal.OFF, dal.ON)},
						"3": {Number: 3, Items: statuses(dal.ON, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 29, 8, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 888,
					Groups: map[string]string{
						"1": "old",
						"2": "29 жовтня:NY", // hash matches, no change
						"3": "old",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 29 жовтня:
Група 1:
  🟢 Заживлено: 00:00 - 12:00;
  🔴 Відключено: 12:00 - 24:00;

Група 3:
  🟢 Заживлено: 00:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"1": "29 жовтня:YN",
					"3": "29 жовтня:YY",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "complex pattern - alternating statuses",
			fields: fields{
				date: "30 жовтня",
				shutdowns: dal.Shutdowns{
					Date: "30 жовтня",
					Periods: periods(
						"00:00", "04:00",
						"04:00", "08:00",
						"08:00", "12:00",
						"12:00", "16:00",
						"16:00", "20:00",
						"20:00", "24:00",
					),
					Groups: map[string]dal.ShutdownGroup{
						"11": {Number: 11, Items: statuses(dal.ON, dal.OFF, dal.ON, dal.OFF, dal.ON, dal.OFF)},
					},
				},
				now: time.Date(2024, 10, 30, 15, 0, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 999,
					Groups: map[string]string{
						"11": "",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 30 жовтня:
Група 11:
  🟢 Заживлено: 16:00 - 20:00;
  🔴 Відключено: 12:00 - 16:00; 20:00 - 24:00;

`,
				UpdatedGroups: map[string]string{
					"11": "30 жовтня:YNYNYN",
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "very early morning - 00:30",
			fields: fields{
				date: "31 жовтня",
				shutdowns: dal.Shutdowns{
					Date: "31 жовтня",
					Periods: periods(
						"00:00", "00:30",
						"00:30", "06:00",
						"06:00", "24:00",
					),
					Groups: map[string]dal.ShutdownGroup{
						"12": {Number: 12, Items: statuses(dal.OFF, dal.MAYBE, dal.ON)},
					},
				},
				now: time.Date(2024, 10, 31, 0, 30, 0, 0, kyiv),
			},
			args: args{
				sub: dal.Subscription{
					ChatID: 1001,
					Groups: map[string]string{
						"12": "hash",
					},
				},
			},
			want: service.Message{
				Text: `Графік стабілізаційних відключень:

📅 31 жовтня:
Група 12:
  🟢 Заживлено: 06:00 - 24:00;
  🟡 Можливо заживлено: 00:30 - 06:00;

`,
				UpdatedGroups: map[string]string{
					"12": "31 жовтня:NMY",
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := service.NewMessageBuilder(tt.fields.date, tt.fields.shutdowns, tt.fields.now)
			got, err := mb.Build(tt.args.sub)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
