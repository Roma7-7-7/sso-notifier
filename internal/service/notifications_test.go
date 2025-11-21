package service_test

import (
	"fmt"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func TestNotifications_NotifyShutdownUpdates(t *testing.T) {
	nowYear := 2025
	nowMonth := time.November
	nowDay := 20
	today := dal.Date{Year: nowYear, Month: nowMonth, Day: nowDay}
	tomorrow := dal.Date{Year: nowYear, Month: nowMonth, Day: nowDay + 1}
	now := time.Date(nowYear, nowMonth, nowDay, 0, 0, 0, 0, time.UTC)

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
	defaultSubscription := testutil.NewSubscription(chatID).
		WithGroups("1", "3", "5", "7", "9", "11").
		Build()

	singleSubscription := testutil.NewSubscription(chatID).WithGroups("1").Build()

	type fields struct {
		shutdowns     func(*gomock.Controller) service.ShutdownsStore
		subscriptions func(*gomock.Controller) service.SubscriptionsStore
		notifications func(*gomock.Controller) service.NotificationsStore
		telegram      func(*gomock.Controller) service.TelegramClient
		clock         service.Clock
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_no_tomorrow_schedule",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					state := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						WithHash("3", "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
						WithHash("7", "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM").
						WithHash("11", "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
						Build()

					res.EXPECT().PutNotificationState(state).Return(nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any())
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_with_tomorrow_schedule",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					todayState := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						WithHash("3", "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
						WithHash("7", "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM").
						WithHash("11", "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
						Build()
					tomorrowState := testutil.NewNotificationState(chatID, tomorrow).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						WithHash("3", "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
						WithHash("7", "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM").
						WithHash("11", "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
						Build()

					res.EXPECT().PutNotificationState(todayState).Return(nil)
					res.EXPECT().PutNotificationState(tomorrowState).Return(nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any())
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_with_get_tomorrow_schedule_failure",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, assert.AnError)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					todayState := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						Build()

					res.EXPECT().PutNotificationState(todayState).Return(nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any())
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_with_partial_changes",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					state := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYNYYMNNNNNNMYYYYYYMNNNNNNM").
						Build()
					res.EXPECT().GetNotificationState(chatID, today).Return(state, true, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					state = testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY"). // was not present
						WithHash("3", "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN"). // not changed
						WithHash("7", "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM"). // changed
						WithHash("11", "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
						Build()

					res.EXPECT().PutNotificationState(state).Return(nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any())
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_without_changes",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{defaultSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					state := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						WithHash("3", "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
						WithHash("5", "YYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNN").
						WithHash("7", "MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN").
						WithHash("9", "NNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNM").
						WithHash("11", "NNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYY").
						Build()
					res.EXPECT().GetNotificationState(chatID, today).Return(state, true, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)

					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_tomorrow_state",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(defaultShutdowns, true, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					todayState := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						Build()
					tomorrowState := testutil.NewNotificationState(chatID, tomorrow).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						Build()

					res.EXPECT().PutNotificationState(todayState).Return(nil)
					res.EXPECT().PutNotificationState(tomorrowState).Return(assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(nil)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_state",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					todayState := testutil.NewNotificationState(chatID, today).
						WithSentAt(now).
						WithHash("1", "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
						Build()

					res.EXPECT().PutNotificationState(todayState).Return(assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(nil)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_send_message",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(assert.AnError)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_send_message_forbidden",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					res.EXPECT().Purge(chatID).Return(nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					res.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(telegram.ErrForbidden)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_tomorrow_notification",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
					res.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_todays_notification",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{singleSubscription}, nil)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					res.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, assert.AnError)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_all_subscriptions",
			fields: fields{
				shutdowns: func(ctrl *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(ctrl)
					res.EXPECT().GetShutdowns(today).Return(defaultShutdowns, true, nil)
					res.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
					return res
				},
				subscriptions: func(ctrl *gomock.Controller) service.SubscriptionsStore {
					res := mocks.NewMockSubscriptionsStore(ctrl)
					res.EXPECT().GetAllSubscriptions().Return(nil, assert.AnError)
					return res
				},
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get all subscriptions: "),
		},
		{
			name: "error_get_shutdown_error",
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
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get shutdowns table for today: "),
		},
		{
			name: "error_no_shutdowns_yet",
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
				notifications: func(ctrl *gomock.Controller) service.NotificationsStore {
					res := mocks.NewMockNotificationsStore(ctrl)
					return res
				},
				telegram: func(ctrl *gomock.Controller) service.TelegramClient {
					res := mocks.NewMockTelegramClient(ctrl)
					return res
				},
				clock: clock.NewMock(now),
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := service.NewNotifications(
				tt.fields.shutdowns(ctrl),
				tt.fields.subscriptions(ctrl),
				tt.fields.notifications(ctrl),
				tt.fields.telegram(ctrl),
				tt.fields.clock,
				time.Hour,
				slog.New(slog.DiscardHandler),
			)
			tt.wantErr(t, s.NotifyShutdownUpdates(t.Context()), "NotifyShutdownUpdates(_)")
		})
	}
}
