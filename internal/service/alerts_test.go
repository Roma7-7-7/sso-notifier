package service_test

import (
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
	"github.com/Roma7-7-7/telegram"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
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
						"1": {},
						"2": {},
						"3": {},
						"4": {},
						//"5":  {},
						"6":  {},
						"7":  {},
						"8":  {},
						"9":  {},
						"10": {},
						//"11": {},
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
				time.UTC,
				slog.New(slog.DiscardHandler),
			)

			tt.wantErr(t, svc.NotifyPowerSupplyChanges(t.Context()), "NotifyPowerSupplyChanges(_)")
		})
	}
}
