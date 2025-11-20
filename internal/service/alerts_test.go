package service_test

import (
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func TestAlerts_NotifyPowerSupplyChanges(t *testing.T) {
	nowYear := 2025
	nowMonth := time.November
	nowDay := 20
	today := dal.Date{Year: nowYear, Month: nowMonth, Day: nowDay}

	defaultShutdowns := testutil.NewShutdowns().
		WithDate(fmt.Sprintf("%d-%2d-%2d", nowYear, nowMonth, nowDay)).
		WithGroup(1, "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
		WithGroup(2, "YYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYM").
		WithGroup(3, "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
		WithGroup(4, "YYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNN").
		WithGroup(5, "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
		WithGroup(6, "YMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNN").
		WithGroup(7, "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
		WithGroup(8, "NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN").
		WithGroup(9, "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM").
		WithGroup(10, "NNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMY").
		WithGroup(11, "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
		WithGroup(12, "NNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYY").
		Build()

	const chatID = int64(123)
	chatIDStr := strconv.FormatInt(chatID, 10)
	defaultSubscription := dal.Subscription{
		ChatID: chatID,
		Groups: map[string]struct{}{
			"1":  {},
			"2":  {},
			"3":  {},
			"4":  {},
			"5":  {},
			"6":  {},
			"7":  {},
			"8":  {},
			"9":  {},
			"10": {},
			"11": {},
			"12": {},
		},
		Settings: map[dal.SettingKey]interface{}{
			dal.SettingNotifyOn:    true,
			dal.SettingNotifyOff:   true,
			dal.SettingNotifyMaybe: true,
		},
	}

	singleGroupSubscription := defaultSubscription
	singleGroupSubscription.Groups = map[string]struct{}{
		"5": {},
	}

	type fields struct {
		shutdowns     func(*gomock.Controller) service.ShutdownsStore
		subscriptions func(*gomock.Controller) service.SubscriptionsStore
		store         func(*gomock.Controller) service.AlertsStore
		telegram      func(*gomock.Controller) service.TelegramClient
		clock         func() service.Clock
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_M_11")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_N_12")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_M_4")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_M_11"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_N_12"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_M_4"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 4, 11:
🟡 Можливе відключення/відновлення електропостачання об 12:00

Група 5:
🟢 Відновлення електропостачання об 12:00

Група 12:
🔴 Відключення електропостачання об 12:00`).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_at_11:50_part_of_alerts_already_sent",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_M_11")).Return(time.Time{}, true, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_N_12")).Return(time.Time{}, true, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_M_4")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_M_4"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 4:
🟡 Можливе відключення/відновлення електропостачання об 12:00

Група 5:
🟢 Відновлення електропостачання об 12:00`).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_at_11:50_groups_5_11_not_subscribed",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					sub := defaultSubscription
					sub.Groups = map[string]struct{}{
						"1":  {},
						"2":  {},
						"3":  {},
						"4":  {},
						"6":  {},
						"7":  {},
						"8":  {},
						"9":  {},
						"10": {},
						"12": {},
					}
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{sub}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_N_12")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_M_4")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_N_12"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_M_4"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 4:
🟡 Можливе відключення/відновлення електропостачання об 12:00

Група 12:
🔴 Відключення електропостачання об 12:00`).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_at_12:20",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_M_10")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_M_3")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_Y_4")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_N_11")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_M_10"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_M_3"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_Y_4"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_N_11"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Групи 3, 10:
🟡 Можливе відключення/відновлення електропостачання об 12:30

Група 4:
🟢 Відновлення електропостачання об 12:30

Група 11:
🔴 Відключення електропостачання об 12:30`).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_at_12:20_maybe_disabled",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					sub := defaultSubscription
					sub.Settings[dal.SettingNotifyMaybe] = false
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{sub}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_Y_4")).Return(time.Time{}, false, nil)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:30_N_11")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_Y_4"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:30_N_11"), time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, `⚠️ Увага! Згідно з графіком Чернівціобленерго незабаром зміниться електропостачання.

Група 4:
🟢 Відновлення електропостачання об 12:30

Група 11:
🔴 Відключення електропостачання об 12:30`).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 12, 20, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "out_of_tolerance_window_12:26",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 12, 26, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "before_notification_window",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 5, 26, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "after_notification_window",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 23, 26, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_alert_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleGroupSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, nil)
					res.EXPECT().PutAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5"), time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC)).Return(assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(nil)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_send_message_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleGroupSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(assert.AnError)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_send_message_blocked_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleGroupSubscription}, nil)
					res.EXPECT().Purge(chatID).Return(nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(telegram.ErrForbidden)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_alert_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleGroupSubscription}, nil)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					res.EXPECT().GetAlert(dal.AlertKey("123_2025-11-20_12:00_Y_5")).Return(time.Time{}, false, assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_subscriptions_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return(nil, assert.AnError)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get all subscriptions: "),
		},
		{
			name: "error_no_shutdowns_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_shutdowns_at_11:50",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(dal.Shutdowns{}, false, assert.AnError)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					return res
				},
				store: func(ctrl *gomock.Controller) service.AlertsStore {
					res := mocks.NewMockAlertsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: func() service.Clock {
					return clock.NewMock(time.Date(2025, time.November, 20, 11, 50, 0, 0, time.UTC))
				},
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get shutdowns: "),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svc := service.NewAlerts(
				tt.fields.shutdowns(ctrl),
				tt.fields.subscriptions(ctrl),
				tt.fields.store(ctrl),
				tt.fields.telegram(ctrl),
				tt.fields.clock(),
				slog.New(slog.DiscardHandler),
			)

			tt.wantErr(t, svc.NotifyPowerSupplyChanges(t.Context()), "NotifyPowerSupplyChanges(_)")
		})
	}
}

func TestPreparePowerSupplyChangeAlerts(t *testing.T) {
	baseShutdowns := testutil.NewShutdowns().
		WithDate("2025-11-20").
		WithGroup(1, "YYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNN").
		WithGroup(2, "NNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYY").
		WithGroup(3, "YYYYYYMMMMMYYYYYYMMMMMYYYYYYMMMMMYYYYYYMMMMMYYYY").
		Build()

	tests := []struct {
		name      string
		shutdowns dal.Shutdowns
		now       time.Time
		target    time.Time
		want      []service.Alert
		wantErr   bool
	}{
		{
			name:      "alerts_at_12:00_within_tolerance",
			shutdowns: baseShutdowns,
			now:       time.Date(2025, 11, 20, 11, 50, 0, 0, time.UTC),
			target:    time.Date(2025, 11, 20, 12, 0, 0, 0, time.UTC),
			want: []service.Alert{
				{GroupNum: "1", Date: "2025-11-20", StartTime: "12:00", Status: dal.ON},
				{GroupNum: "2", Date: "2025-11-20", StartTime: "12:00", Status: dal.OFF},
			},
			wantErr: false,
		},
		{
			name:      "no_alerts_continuation_of_same_status",
			shutdowns: baseShutdowns,
			now:       time.Date(2025, 11, 20, 0, 20, 0, 0, time.UTC),
			target:    time.Date(2025, 11, 20, 0, 30, 0, 0, time.UTC),
			want:      []service.Alert{},
			wantErr:   false,
		},
		{
			name:      "alerts_at_period_boundary",
			shutdowns: baseShutdowns,
			now:       time.Date(2025, 11, 20, 11, 55, 0, 0, time.UTC),
			target:    time.Date(2025, 11, 20, 12, 5, 0, 0, time.UTC),
			want: []service.Alert{
				{GroupNum: "1", Date: "2025-11-20", StartTime: "12:00", Status: dal.ON},
				{GroupNum: "2", Date: "2025-11-20", StartTime: "12:00", Status: dal.OFF},
			},
			wantErr: false,
		},
		{
			name:      "outside_tolerance_window_too_early",
			shutdowns: baseShutdowns,
			now:       time.Date(2025, 11, 20, 11, 44, 0, 0, time.UTC),
			target:    time.Date(2025, 11, 20, 11, 54, 0, 0, time.UTC),
			want:      nil,
			wantErr:   false,
		},
		{
			name:      "outside_tolerance_window_too_late",
			shutdowns: baseShutdowns,
			now:       time.Date(2025, 11, 20, 12, 6, 0, 0, time.UTC),
			target:    time.Date(2025, 11, 20, 12, 16, 0, 0, time.UTC),
			want:      nil,
			wantErr:   false,
		},
		{
			name: "multiple_groups_same_status_change",
			shutdowns: testutil.NewShutdowns().
				WithDate("2025-11-20").
				WithGroup(1, "YYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNN").
				WithGroup(2, "YYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNN").
				WithGroup(3, "YYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNNYYYYYYNNNNNN").
				Build(),
			now:    time.Date(2025, 11, 20, 11, 50, 0, 0, time.UTC),
			target: time.Date(2025, 11, 20, 12, 0, 0, 0, time.UTC),
			want: []service.Alert{
				{GroupNum: "1", Date: "2025-11-20", StartTime: "12:00", Status: dal.ON},
				{GroupNum: "2", Date: "2025-11-20", StartTime: "12:00", Status: dal.ON},
				{GroupNum: "3", Date: "2025-11-20", StartTime: "12:00", Status: dal.ON},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.PreparePowerSupplyChangeAlerts(tt.shutdowns, tt.now, tt.target)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.ElementsMatch(t, tt.want, got)
			}
		})
	}
}

func TestParseTimeToMinutes(t *testing.T) {
	tests := []struct {
		name    string
		timeStr string
		want    int
		wantErr bool
	}{
		{
			name:    "midnight",
			timeStr: "00:00",
			want:    0,
			wantErr: false,
		},
		{
			name:    "morning",
			timeStr: "08:30",
			want:    510,
			wantErr: false,
		},
		{
			name:    "noon",
			timeStr: "12:00",
			want:    720,
			wantErr: false,
		},
		{
			name:    "evening",
			timeStr: "18:45",
			want:    1125,
			wantErr: false,
		},
		{
			name:    "end_of_day",
			timeStr: "24:00",
			want:    1440,
			wantErr: false,
		},
		{
			name:    "single_digit_hour",
			timeStr: "9:15",
			want:    555,
			wantErr: false,
		},
		{
			name:    "invalid_format_no_colon",
			timeStr: "1030",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid_format_too_many_parts",
			timeStr: "10:30:45",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid_hour",
			timeStr: "XX:30",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid_minute",
			timeStr: "10:YY",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty_string",
			timeStr: "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.ParseTimeToMinutes(tt.timeStr)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsWithinNotificationWindow(t *testing.T) {
	tests := []struct {
		name string
		hour int
		want bool
	}{
		{name: "before_window_midnight", hour: 0, want: false},
		{name: "before_window_5am", hour: 5, want: false},
		{name: "start_of_window_6am", hour: 6, want: true},
		{name: "mid_morning_9am", hour: 9, want: true},
		{name: "noon", hour: 12, want: true},
		{name: "evening_6pm", hour: 18, want: true},
		{name: "late_evening_10pm", hour: 22, want: true},
		{name: "end_of_window_11pm", hour: 23, want: false},
		{name: "after_window_midnight", hour: 24, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.IsWithinNotificationWindow(tt.hour)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetSettingKeyForStatus(t *testing.T) {
	tests := []struct {
		name   string
		status dal.Status
		want   dal.SettingKey
	}{
		{
			name:   "off_status",
			status: dal.OFF,
			want:   dal.SettingNotifyOff,
		},
		{
			name:   "maybe_status",
			status: dal.MAYBE,
			want:   dal.SettingNotifyMaybe,
		},
		{
			name:   "on_status",
			status: dal.ON,
			want:   dal.SettingNotifyOn,
		},
		{
			name:   "unknown_status",
			status: dal.Status("UNKNOWN"),
			want:   dal.SettingKey(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.GetSettingKeyForStatus(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsOutageStart(t *testing.T) {
	tests := []struct {
		name   string
		items  []dal.Status
		index  int
		status dal.Status
		want   bool
	}{
		{
			name:   "start_at_beginning_off",
			items:  []dal.Status{dal.OFF, dal.OFF, dal.ON},
			index:  0,
			status: dal.OFF,
			want:   true,
		},
		{
			name:   "start_after_different_status_off",
			items:  []dal.Status{dal.ON, dal.OFF, dal.OFF},
			index:  1,
			status: dal.OFF,
			want:   true,
		},
		{
			name:   "not_start_continuation_off",
			items:  []dal.Status{dal.OFF, dal.OFF, dal.OFF},
			index:  1,
			status: dal.OFF,
			want:   false,
		},
		{
			name:   "start_after_maybe_to_off",
			items:  []dal.Status{dal.MAYBE, dal.OFF, dal.OFF},
			index:  1,
			status: dal.OFF,
			want:   true,
		},
		{
			name:   "start_after_off_to_on",
			items:  []dal.Status{dal.OFF, dal.ON, dal.ON},
			index:  1,
			status: dal.ON,
			want:   true,
		},
		{
			name:   "start_maybe_after_on",
			items:  []dal.Status{dal.ON, dal.MAYBE, dal.OFF},
			index:  1,
			status: dal.MAYBE,
			want:   true,
		},
		{
			name:   "not_start_wrong_status",
			items:  []dal.Status{dal.ON, dal.OFF, dal.OFF},
			index:  1,
			status: dal.ON,
			want:   false,
		},
		{
			name:   "invalid_index_negative",
			items:  []dal.Status{dal.OFF, dal.ON},
			index:  -1,
			status: dal.OFF,
			want:   false,
		},
		{
			name:   "invalid_index_out_of_bounds",
			items:  []dal.Status{dal.OFF, dal.ON},
			index:  5,
			status: dal.OFF,
			want:   false,
		},
		{
			name:   "empty_items",
			items:  []dal.Status{},
			index:  0,
			status: dal.OFF,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.IsOutageStart(tt.items, tt.index, tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFindPeriodIndex(t *testing.T) {
	periods := []dal.Period{
		{From: "00:00", To: "00:30"},
		{From: "00:30", To: "01:00"},
		{From: "01:00", To: "01:30"},
		{From: "09:00", To: "09:30"},
		{From: "09:30", To: "10:00"},
		{From: "23:30", To: "24:00"},
	}

	tests := []struct {
		name       string
		periods    []dal.Period
		targetTime time.Time
		wantIndex  int
		wantErr    bool
	}{
		{
			name:       "exact_start_first_period",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 0, 0, 0, 0, time.UTC),
			wantIndex:  0,
			wantErr:    false,
		},
		{
			name:       "middle_of_first_period",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 0, 15, 0, 0, time.UTC),
			wantIndex:  0,
			wantErr:    false,
		},
		{
			name:       "exact_start_second_period",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 0, 30, 0, 0, time.UTC),
			wantIndex:  1,
			wantErr:    false,
		},
		{
			name:       "middle_period",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 9, 15, 0, 0, time.UTC),
			wantIndex:  3,
			wantErr:    false,
		},
		{
			name:       "last_period",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 23, 45, 0, 0, time.UTC),
			wantIndex:  5,
			wantErr:    false,
		},
		{
			name:       "one_minute_before_end",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 23, 59, 0, 0, time.UTC),
			wantIndex:  5,
			wantErr:    false,
		},
		{
			name:       "time_between_periods",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 5, 0, 0, 0, time.UTC),
			wantIndex:  0,
			wantErr:    true,
		},
		{
			name:       "time_after_all_periods",
			periods:    periods,
			targetTime: time.Date(2025, 11, 20, 5, 0, 0, 0, time.UTC),
			wantIndex:  0,
			wantErr:    true,
		},
		{
			name:       "empty_periods",
			periods:    []dal.Period{},
			targetTime: time.Date(2025, 11, 20, 12, 0, 0, 0, time.UTC),
			wantIndex:  0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, err := service.FindPeriodIndex(tt.periods, tt.targetTime)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantIndex, gotIndex)
			}
		})
	}
}
