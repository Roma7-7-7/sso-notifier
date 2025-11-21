package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
)

func TestPowerSupplyScheduleMessageBuilder_Build(t *testing.T) {
	beginningOfDay := time.Date(2025, time.November, 10, 0, 30, 0, 0, time.UTC)
	middleOfDay := time.Date(2025, time.November, 10, 12, 0, 0, 0, time.UTC)
	endOfDay := time.Date(2025, time.November, 10, 23, 30, 0, 0, time.UTC)

	todayDate := middleOfDay.Format(time.DateOnly)
	tomorrowDate := middleOfDay.AddDate(0, 0, 1).Format(time.DateOnly)
	defaultTodayShutdowns := testutil.NewShutdowns().
		WithDate(todayDate).
		WithStubGroups().
		Build()
	const defaultTomorrowHash = "YYYYYYYYMNNNNNNNNMYYYYYYYYMNNNNNNNNMYYYYYYYYMNNN"
	defaultTomorrowShutdowns := testutil.NewShutdowns().
		WithDate(tomorrowDate).
		WithGroup(1, defaultTomorrowHash).
		WithGroup(2, defaultTomorrowHash).
		WithGroup(3, defaultTomorrowHash).
		WithGroup(4, defaultTomorrowHash).
		WithGroup(5, defaultTomorrowHash).
		WithGroup(6, defaultTomorrowHash).
		WithGroup(7, defaultTomorrowHash).
		WithGroup(8, defaultTomorrowHash).
		WithGroup(9, defaultTomorrowHash).
		WithGroup(10, defaultTomorrowHash).
		WithGroup(11, defaultTomorrowHash).
		WithGroup(12, defaultTomorrowHash).
		Build()

	type fields struct {
		shutdowns        dal.Shutdowns
		nextDayShutdowns *dal.Shutdowns
		now              func() time.Time
	}
	type args struct {
		sub              dal.Subscription
		todayState       dal.NotificationState
		tomorrowState    dal.NotificationState
		withPeriodRanges bool
	}
	tests := []struct {
		name       string
		fields     fields
		args       args
		want       service.PowerSupplyScheduleMessage
		wantLinear service.PowerSupplyScheduleMessage
		wantErr    assert.ErrorAssertionFunc
	}{
		// ===================== Single group ===================== //
		{
			name: "success_single_group_beginning_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 00:30 | 🟡 03:30 | 🟢 04:00 | 🟡 07:00 | 🔴 07:30 | 🟡 10:30 | 🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_single_group_middle_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				withPeriodRanges: true,
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 14:30 - 17:30; 21:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 11:00 - 14:00 | 🟡 14:00 - 14:30 | 🔴 14:30 - 17:30 | 🟡 17:30 - 18:00 | 🟢 18:00 - 21:00 | 🟡 21:00 - 21:30 | 🔴 21:30 - 24:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_single_group_end_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return endOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 21:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 21:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},

		// ===================== Multiple groups ===================== //
		{
			name: "success_multiple_groups_beginning_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				withPeriodRanges: true,
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 00:00 - 03:00; 07:00 - 10:00; 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 03:00 - 03:30; 06:30 - 07:00; 10:00 - 10:30; 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 03:30 - 06:30; 10:30 - 13:30; 17:30 - 20:30;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 00:30 - 03:30 | 🟡 03:30 - 04:00 | 🟢 04:00 - 07:00 | 🟡 07:00 - 07:30 | 🔴 07:30 - 10:30 | 🟡 10:30 - 11:00 | 🟢 11:00 - 14:00 | 🟡 14:00 - 14:30 | 🔴 14:30 - 17:30 | 🟡 17:30 - 18:00 | 🟢 18:00 - 21:00 | 🟡 21:00 - 21:30 | 🔴 21:30 - 24:00

Група 5: 
🟢 00:00 - 03:00 | 🟡 03:00 - 03:30 | 🔴 03:30 - 06:30 | 🟡 06:30 - 07:00 | 🟢 07:00 - 10:00 | 🟡 10:00 - 10:30 | 🔴 10:30 - 13:30 | 🟡 13:30 - 14:00 | 🟢 14:00 - 17:00 | 🟡 17:00 - 17:30 | 🔴 17:30 - 20:30 | 🟡 20:30 - 21:00 | 🟢 21:00 - 24:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_multiple_groups_middle_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 14:30 - 17:30; 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 10:30 - 13:30; 17:30 - 20:30;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

Група 5: 
🔴 10:30 | 🟡 13:30 | 🟢 14:00 | 🟡 17:00 | 🔴 17:30 | 🟡 20:30 | 🟢 21:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_multiple_groups_end_of_day",
			fields: fields{
				shutdowns: defaultTodayShutdowns,
				now: func() time.Time {
					return endOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 21:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 21:30

Група 5: 
🟢 21:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		// =============== Single group with tomorrow =============== //
		{
			name: "success_single_group_with_tomorrow_beginning_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 00:30 | 🟡 03:30 | 🟢 04:00 | 🟡 07:00 | 🔴 07:30 | 🟡 10:30 | 🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_single_group_with_tomorrow_middle_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 14:30 - 17:30; 21:30 - 24:00;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_single_group_with_tomorrow_end_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return endOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 21:30 - 24:00;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 21:30


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},

		// =============== Multiple groups with tomorrow =============== //
		{
			name: "success_multiple_groups_with_tomorrow_beginning_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 00:00 - 03:00; 07:00 - 10:00; 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 03:00 - 03:30; 06:30 - 07:00; 10:00 - 10:30; 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 03:30 - 06:30; 10:30 - 13:30; 17:30 - 20:30;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

Група 5:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 00:30 | 🟡 03:30 | 🟢 04:00 | 🟡 07:00 | 🔴 07:30 | 🟡 10:30 | 🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

Група 5: 
🟢 00:00 | 🟡 03:00 | 🔴 03:30 | 🟡 06:30 | 🟢 07:00 | 🟡 10:00 | 🔴 10:30 | 🟡 13:30 | 🟢 14:00 | 🟡 17:00 | 🔴 17:30 | 🟡 20:30 | 🟢 21:00


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

Група 5: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_multiple_groups_with_tomorrow_middle_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 14:30 - 17:30; 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 10:30 - 13:30; 17:30 - 20:30;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

Група 5:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

Група 5: 
🔴 10:30 | 🟡 13:30 | 🟢 14:00 | 🟡 17:00 | 🔴 17:30 | 🟡 20:30 | 🟢 21:00


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

Група 5: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_multiple_groups_with_tomorrow_end_of_day",
			fields: fields{
				shutdowns:        defaultTodayShutdowns,
				nextDayShutdowns: &defaultTomorrowShutdowns,
				now: func() time.Time {
					return endOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 21:00 - 24:00;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

Група 5:
  🟢 Заживлено: 00:00 - 04:00; 09:00 - 13:00; 18:00 - 22:00;
  🟡 Можливо заживлено: 04:00 - 04:30; 08:30 - 09:00; 13:00 - 13:30; 17:30 - 18:00; 22:00 - 22:30;
  🔴 Відключено: 04:30 - 08:30; 13:30 - 17:30; 22:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 21:30

Група 5: 
🟢 21:00


📅 2025-11-11:
Група 4: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

Група 5: 
🟢 00:00 | 🟡 04:00 | 🔴 04:30 | 🟡 08:30 | 🟢 09:00 | 🟡 13:00 | 🔴 13:30 | 🟡 17:30 | 🟢 18:00 | 🟡 22:00 | 🔴 22:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": defaultTomorrowHash,
					"5": defaultTomorrowHash,
				},
			},
			wantErr: assert.NoError,
		},
		// ===================== No changes ===================== //
		{
			name: "single_group_no_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					Build(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple_groups_no_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					Build(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "single_group_with_tomorrow_no_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					Build(),
				nextDayShutdowns: testutil.NewShutdowns().WithDate(tomorrowDate).
					WithGroup(4, testutil.AllStatesOnHash).
					BuildPointer(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple_groups_with_tomorrow_no_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					Build(),
				nextDayShutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					BuildPointer(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		// ===================== Partial changes ===================== //
		{
			name: "multiple_groups_partial_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					Build(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
						"5": testutil.AllStatesOffHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 5:
  🟢 Заживлено: 00:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"5": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 5: 
🟢 00:00

`,
				TodayUpdatedGroups: map[string]string{
					"5": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "single_group_with_tomorrow_partial_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					Build(),
				nextDayShutdowns: testutil.NewShutdowns().WithDate(tomorrowDate).
					WithGroup(4, testutil.AllStatesOffHash).
					BuildPointer(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-11:
Група 4:
  🔴 Відключено: 00:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{},
				TomorrowUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOffHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-11:
Група 4: 
🔴 00:00

`,
				TodayUpdatedGroups: map[string]string{},
				TomorrowUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOffHash,
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "multiple_groups_with_tomorrow_partial_changes_middle_of_day",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					Build(),
				nextDayShutdowns: testutil.NewShutdowns().WithDate(tomorrowDate).
					WithGroup(4, testutil.AllStatesOnHash).
					WithGroup(5, testutil.AllStatesOnHash).
					BuildPointer(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOffHash,
						"5": testutil.AllStatesOnHash,
					},
				},
				tomorrowState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOffHash,
						"5": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 00:00 - 24:00;


📅 2025-11-11:
Група 4:
  🟢 Заживлено: 00:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 00:00


📅 2025-11-11:
Група 4: 
🟢 00:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
			},
			wantErr: assert.NoError,
		},
		// ===================== Edge cases ===================== //
		{
			name: "success_group_numeric_sorting_with_double_digits",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(1, testutil.StubGroupHashes[1]).
					WithGroup(2, testutil.StubGroupHashes[2]).
					WithGroup(11, testutil.StubGroupHashes[11]).
					WithGroup(12, testutil.StubGroupHashes[12]).
					Build(),
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("12", "1", "11", "2").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"1":  testutil.AllStatesOnHash,
						"2":  testutil.AllStatesOnHash,
						"11": testutil.AllStatesOnHash,
						"12": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 1:
  🟢 Заживлено: 00:00 - 03:00; 07:00 - 10:00; 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 03:00 - 03:30; 06:30 - 07:00; 10:00 - 10:30; 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 03:30 - 06:30; 10:30 - 13:30; 17:30 - 20:30;

Група 2:
  🟢 Заживлено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;

Група 11:
  🟢 Заживлено: 03:30 - 06:30; 10:30 - 13:30; 17:30 - 20:30;
  🟡 Можливо заживлено: 03:00 - 03:30; 06:30 - 07:00; 10:00 - 10:30; 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 00:00 - 03:00; 07:00 - 10:00; 14:00 - 17:00; 21:00 - 24:00;

Група 12:
  🟢 Заживлено: 04:00 - 07:00; 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 03:30 - 04:00; 07:00 - 07:30; 10:30 - 11:00; 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 00:30 - 03:30; 07:30 - 10:30; 14:30 - 17:30; 21:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"1":  testutil.StubGroupHashes[1],
					"2":  testutil.StubGroupHashes[2],
					"11": testutil.StubGroupHashes[11],
					"12": testutil.StubGroupHashes[12],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 1: 
🟢 00:00 | 🟡 03:00 | 🔴 03:30 | 🟡 06:30 | 🟢 07:00 | 🟡 10:00 | 🔴 10:30 | 🟡 13:30 | 🟢 14:00 | 🟡 17:00 | 🔴 17:30 | 🟡 20:30 | 🟢 21:00

Група 2: 
🟢 00:30 | 🟡 03:30 | 🔴 04:00 | 🟡 07:00 | 🟢 07:30 | 🟡 10:30 | 🔴 11:00 | 🟡 14:00 | 🟢 14:30 | 🟡 17:30 | 🔴 18:00 | 🟡 21:00 | 🟢 21:30

Група 11: 
🔴 00:00 | 🟡 03:00 | 🟢 03:30 | 🟡 06:30 | 🔴 07:00 | 🟡 10:00 | 🟢 10:30 | 🟡 13:30 | 🔴 14:00 | 🟡 17:00 | 🟢 17:30 | 🟡 20:30 | 🔴 21:00

Група 12: 
🔴 00:30 | 🟡 03:30 | 🟢 04:00 | 🟡 07:00 | 🔴 07:30 | 🟡 10:30 | 🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

`,
				TodayUpdatedGroups: map[string]string{
					"1":  testutil.StubGroupHashes[1],
					"2":  testutil.StubGroupHashes[2],
					"11": testutil.StubGroupHashes[11],
					"12": testutil.StubGroupHashes[12],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "subscription_with_non_existent_groups",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.StubGroupHashes[4]).
					WithGroup(5, testutil.StubGroupHashes[5]).
					Build(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4", "5", "99").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4":  testutil.AllStatesOnHash,
						"5":  testutil.AllStatesOnHash,
						"99": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 11:00 - 14:00; 18:00 - 21:00;
  🟡 Можливо заживлено: 14:00 - 14:30; 17:30 - 18:00; 21:00 - 21:30;
  🔴 Відключено: 14:30 - 17:30; 21:30 - 24:00;

Група 5:
  🟢 Заживлено: 14:00 - 17:00; 21:00 - 24:00;
  🟡 Можливо заживлено: 13:30 - 14:00; 17:00 - 17:30; 20:30 - 21:00;
  🔴 Відключено: 10:30 - 13:30; 17:30 - 20:30;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 11:00 | 🟡 14:00 | 🔴 14:30 | 🟡 17:30 | 🟢 18:00 | 🟡 21:00 | 🔴 21:30

Група 5: 
🔴 10:30 | 🟡 13:30 | 🟢 14:00 | 🟡 17:00 | 🔴 17:30 | 🟡 20:30 | 🟢 21:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
					"5": testutil.StubGroupHashes[5],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "group_with_only_on_status",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOnHash).
					Build(),
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOffHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🟢 Заживлено: 00:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🟢 00:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOnHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "group_with_only_off_status",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.AllStatesOffHash).
					Build(),
				now: func() time.Time {
					return beginningOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 00:00 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOffHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 00:00

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.AllStatesOffHash,
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "end_of_day_with_remaining_period",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithGroup(4, testutil.StubGroupHashes[4]).
					Build(),
				now: func() time.Time {
					return time.Date(2025, time.November, 10, 23, 59, 59, 0, time.UTC)
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).WithGroups("4").Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{
						"4": testutil.AllStatesOnHash,
					},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4:
  🔴 Відключено: 21:30 - 24:00;

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text: `Графік стабілізаційних відключень:

📅 2025-11-10:
Група 4: 
🔴 21:30

`,
				TodayUpdatedGroups: map[string]string{
					"4": testutil.StubGroupHashes[4],
				},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
		{
			name: "empty_subscription_groups",
			fields: fields{
				shutdowns: testutil.NewShutdowns().WithDate(todayDate).
					WithStubGroups().
					Build(),
				now: func() time.Time {
					return middleOfDay
				},
			},
			args: args{
				sub: testutil.NewSubscription(123).Build(),
				todayState: dal.NotificationState{
					ChatID: 123,
					Hashes: map[string]string{},
				},
			},
			want: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantLinear: service.PowerSupplyScheduleMessage{
				Text:                  ``,
				TodayUpdatedGroups:    map[string]string{},
				TomorrowUpdatedGroups: map[string]string{},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+"_original", func(t *testing.T) {
			mb := service.NewPowerSupplyScheduleMessageBuilder(tt.fields.shutdowns, tt.fields.now())
			lmb := service.NewPowerSupplyScheduleLinearMessageBuilder(tt.fields.shutdowns, tt.fields.now()).
				WithPeriodRanges(tt.args.withPeriodRanges)
			if tt.fields.nextDayShutdowns != nil {
				mb.WithNextDay(*tt.fields.nextDayShutdowns)
				lmb.WithNextDay(*tt.fields.nextDayShutdowns)
			}
			got, err := mb.Build(tt.args.sub, tt.args.todayState, tt.args.tomorrowState)
			if tt.wantErr(t, err, "service.PowerSupplyScheduleMessageBuilder.Build(%v)", tt.args.sub) {
				assert.Equalf(t, tt.want, got, "service.PowerSupplyScheduleMessageBuilder.Build() error = %v, wantErr %v", err, tt.want)
			}
			got, err = lmb.Build(tt.args.sub, tt.args.todayState, tt.args.tomorrowState)
			if tt.wantErr(t, err, "service.PowerSupplyScheduleLinearMessageBuilder.Build(%v)", tt.args.sub) {
				assert.Equalf(t, tt.wantLinear, got, "service.PowerSupplyScheduleLinearMessageBuilder.Build(%v)", tt.want)
			}
		})
	}
}

func TestPowerSupplyChangeMessageBuilder_Build(t *testing.T) {
	type args struct {
		alerts []service.Alert
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty_alerts",
			args: args{
				alerts: []service.Alert{},
			},
			want: "",
		},
		{
			name: "combined",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "2",
						StartTime: "12:20",
						Status:    dal.MAYBE,
					},
					{
						GroupNum:  "3",
						StartTime: "13:00",
						Status:    dal.OFF,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 1:
🟢 Відновлення електропостачання об 12:00

Група 2:
🟡 Можливе відключення/відновлення електропостачання об 12:20

Група 3:
🔴 Відключення електропостачання об 13:00`,
		},
		{
			name: "sort_by_time",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "23:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "1",
						StartTime: "09:00",
						Status:    dal.MAYBE,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 1:
🟡 Можливе відключення/відновлення електропостачання об 09:00

Група 1:
🟢 Відновлення електропостачання об 12:00

Група 1:
🔴 Відключення електропостачання об 23:00`,
		},
		{
			name: "group_by_status_and_time",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.ON,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 1, 2:
🟢 Відновлення електропостачання об 12:00`,
		},
		{
			name: "numeric_group_sorting_at_same_time",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "12",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "11",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.ON,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 1, 2, 11, 12:
🟢 Відновлення електропостачання об 12:00`,
		},
		{
			name: "sort_by_status_priority_same_time_same_group",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.MAYBE,
					},
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 1:
🔴 Відключення електропостачання об 12:00

Група 1:
🟡 Можливе відключення/відновлення електропостачання об 12:00

Група 1:
🟢 Відновлення електропостачання об 12:00`,
		},
		{
			name: "sort_by_min_group_number_same_time_different_status",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "5",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "3",
						StartTime: "12:00",
						Status:    dal.MAYBE,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 2:
🟢 Відновлення електропостачання об 12:00

Група 3:
🟡 Можливе відключення/відновлення електропостачання об 12:00

Група 5:
🔴 Відключення електропостачання об 12:00`,
		},
		{
			name: "group_multiple_groups_same_status_time",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "3",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "5",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 1, 2, 3, 5:
🔴 Відключення електропостачання об 12:00`,
		},
		{
			name: "mixed_grouping_scenario",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "3",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "4",
						StartTime: "13:00",
						Status:    dal.MAYBE,
					},
					{
						GroupNum:  "5",
						StartTime: "13:00",
						Status:    dal.MAYBE,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 1, 2:
🔴 Відключення електропостачання об 12:00

Група 3:
🟢 Відновлення електропостачання об 12:00

Групи 4, 5:
🟡 Можливе відключення/відновлення електропостачання об 13:00`,
		},
		{
			name: "complex_sorting_time_group_status",
			args: args{
				alerts: []service.Alert{
					{
						GroupNum:  "3",
						StartTime: "14:00",
						Status:    dal.MAYBE,
					},
					{
						GroupNum:  "1",
						StartTime: "12:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "2",
						StartTime: "12:00",
						Status:    dal.ON,
					},
					{
						GroupNum:  "11",
						StartTime: "13:00",
						Status:    dal.OFF,
					},
					{
						GroupNum:  "5",
						StartTime: "13:00",
						Status:    dal.OFF,
					},
				},
			},
			want: `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 1:
🔴 Відключення електропостачання об 12:00

Група 2:
🟢 Відновлення електропостачання об 12:00

Групи 5, 11:
🔴 Відключення електропостачання об 13:00

Група 3:
🟡 Можливе відключення/відновлення електропостачання об 14:00`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := service.NewPowerSupplyChangeMessageBuilder()
			assert.Equalf(t, tt.want, b.Build(tt.args.alerts), "Build(%v)", tt.args.alerts)
		})
	}
}
