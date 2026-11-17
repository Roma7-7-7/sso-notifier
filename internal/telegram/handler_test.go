package telegram_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	tb "gopkg.in/telebot.v3"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram/mocks"
)

const chatID = int64(123)

var defaultUser = &tb.User{
	ID: chatID,
}

func TestHandler_Start(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		ctx func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_new_user",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(false, nil)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser)
					ctx.EXPECT().Callback().Return(nil)
					ctx.EXPECT().Send("Привіт! Бажаєте підписатись на оновлення графіку відключень?", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_new_user_with_callback",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(false, nil)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser)
					ctx.EXPECT().Callback().Return(&tb.Callback{})
					ctx.EXPECT().Delete().Return(nil)
					ctx.EXPECT().Send("Привіт! Бажаєте підписатись на оновлення графіку відключень?", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(true, nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"7", "3", "1"}, nil)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser)
					ctx.EXPECT().Callback().Return(nil)
					ctx.EXPECT().Send("Привіт! Ви підписані на групи: 1, 3, 7", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_subscribed_with_callback_delete_error",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(true, nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"7", "3", "1"}, nil)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser).AnyTimes()
					ctx.EXPECT().Callback().Return(&tb.Callback{})
					ctx.EXPECT().Message().Return(&tb.Message{})
					ctx.EXPECT().Delete().Return(assert.AnError)
					ctx.EXPECT().Send("Привіт! Ви підписані на групи: 1, 3, 7", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_subscribed_groups",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(true, nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser)
					ctx.EXPECT().Callback().Return(nil)
					ctx.EXPECT().Send("Щось пішло не так. Будь ласка, спробуйте пізніше.", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_check_if_sub subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(gomock.Any()).Return(false, assert.AnError)
					return res
				},
			},
			args: args{
				ctx: func(ctrl *gomock.Controller) tb.Context {
					ctx := mocks.NewMockTelebotContext(ctrl)
					ctx.EXPECT().Sender().Return(defaultUser)
					ctx.EXPECT().Callback().Return(nil)
					ctx.EXPECT().Send("Щось пішло не так. Будь ласка, спробуйте пізніше.", gomock.Not(gomock.Nil())).Return(nil)
					return ctx
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.Start(tt.args.ctx(ctrl)), "Start")
		})
	}
}

func TestHandler_ManageGroups(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"7", "3", "1"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser)
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_subscribed_groups",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().GetSubscribedGroups(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser)
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.ManageGroups(tt.args.c(ctrl)), "ManageGroups(_)")
		})
	}
}

func TestHandler_ToggleGroupHandler(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		groupNumber string
		ctx         func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantError assert.ErrorAssertionFunc
	}{
		{
			name: "success_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)

					res.EXPECT().ToggleGroupSubscription(chatID, "3").Return(nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"7", "3", "1"}, nil)

					return res
				},
			},
			args: args{
				groupNumber: "3",
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`✅ Підписано на групу 3

Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantError: assert.NoError,
		},
		{
			name: "success_unsubscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)

					res.EXPECT().ToggleGroupSubscription(chatID, "3").Return(nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"7", "1"}, nil)

					return res
				},
			},
			args: args{
				groupNumber: "3",
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`❌ Відписано від групи 3

Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantError: assert.NoError,
		},
		{
			name: "success_unsubscribed_from_all",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)

					res.EXPECT().ToggleGroupSubscription(chatID, "3").Return(nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return(nil, nil)

					return res
				},
			},
			args: args{
				groupNumber: "3",
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Ви відписані від усіх груп`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantError: assert.NoError,
		},
		{
			name: "error_get_subscribed_groups",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)

					res.EXPECT().ToggleGroupSubscription(chatID, "3").Return(nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return(nil, assert.AnError)

					return res
				},
			},
			args: args{
				groupNumber: "3",
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantError: assert.NoError,
		},
		{
			name: "error_toggle_group_subscription",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)

					res.EXPECT().ToggleGroupSubscription(chatID, "3").Return(assert.AnError)

					return res
				},
			},
			args: args{
				groupNumber: "3",
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantError: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			fn := h.ToggleGroupHandler(tt.args.groupNumber)
			tt.wantError(t, fn(tt.args.ctx(ctrl)), "ToggleGroupHandler")
		})
	}
}

func TestHandler_Settings(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`⚙️ Налаштування

Оберіть розділ налаштувань:`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_not_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Налаштування доступні тільки для підписаних користувачів. Спочатку підпишіться на оновлення.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_is_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.Settings(tt.args.c(ctrl)), "Settings(_)")
		})
	}
}

func TestHandler_ToggleSettingHandler(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		settingKey dal.SettingKey
		ctx        func(ctrl *gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_toggle_setting",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyOn, true).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingNotifyOn:    true,
						dal.SettingNotifyMaybe: true,
						dal.SettingNotifyOff:   true,
					}, nil)
					return res
				},
			},
			args: args{
				settingKey: dal.SettingNotifyOn,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`🔔 Попереджати за 10 хвилин до:

ℹ️ Сповіщення надсилаються з 6:00 до 23:00`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_setting",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyOn, true).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				settingKey: dal.SettingNotifyOn,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_toggle_setting",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyOn, true).Return(assert.AnError)
					return res
				},
			},
			args: args{
				settingKey: dal.SettingNotifyOn,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			fn := h.ToggleSettingHandler(tt.args.settingKey)
			tt.wantErr(t, fn(tt.args.ctx(ctrl)), "ToggleSettingHandler(_)")
		})
	}
}

func TestHandler_GetSchedule(t *testing.T) {
	defaultEmergency := func(ctrl *gomock.Controller) telegram.EmergencyStore {
		res := mocks.NewMockEmergencyStore(ctrl)
		res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil).AnyTimes()
		return res
	}

	type fields struct {
		notifications func(*gomock.Controller) telegram.Notifications
		emergency     func(*gomock.Controller) telegram.EmergencyStore
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success",
			fields: fields{
				notifications: func(ctrl *gomock.Controller) telegram.Notifications {
					res := mocks.NewMockNotifications(ctrl)
					res.EXPECT().NotifyPowerSupplySchedule(gomock.Any(), chatID).Return(nil)
					return res
				},
				emergency: defaultEmergency,
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_notify",
			fields: fields{
				notifications: func(ctrl *gomock.Controller) telegram.Notifications {
					res := mocks.NewMockNotifications(ctrl)
					res.EXPECT().NotifyPowerSupplySchedule(gomock.Any(), chatID).Return(assert.AnError)
					return res
				},
				emergency: defaultEmergency,
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send("Щось пішло не так. Будь ласка, спробуйте пізніше.", gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_unsubscribed",
			fields: fields{
				notifications: func(ctrl *gomock.Controller) telegram.Notifications {
					res := mocks.NewMockNotifications(ctrl)
					res.EXPECT().NotifyPowerSupplySchedule(gomock.Any(), chatID).Return(service.ErrSubscriptionNotFound)
					return res
				},
				emergency: defaultEmergency,
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send("Графік електропостачання доступний тільки для підписаних користувачів. Спочатку підпишіться на оновлення.", gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "emergency_mode_returns_emergency_message",
			fields: fields{
				notifications: func(ctrl *gomock.Controller) telegram.Notifications {
					res := mocks.NewMockNotifications(ctrl)
					return res
				},
				emergency: func(ctrl *gomock.Controller) telegram.EmergencyStore {
					res := mocks.NewMockEmergencyStore(ctrl)
					res.EXPECT().GetEmergencyState().Return(dal.EmergencyState{
						Active: true,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send("⚠️⚠️⚠️\nЗапроваджено екстренні відключення по Чернівецькій області. \nГрафіки погодинних відключень тимчасово не діють.\n⚠️⚠️⚠️", gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := gomock.NewController(t)
			defer ctr.Finish()

			h := telegram.NewHandler(
				nil,
				tt.fields.notifications(ctr),
				tt.fields.emergency(ctr),
				12,
				0,
				slog.New(slog.DiscardHandler),
			)
			tt.wantErr(t, h.GetSchedule(tt.args.c(ctr)), "GetSchedule(_)")
		})
	}
}

func TestHandler_SettingsAlerts(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingNotifyOn:    true,
						dal.SettingNotifyMaybe: false,
						dal.SettingNotifyOff:   true,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`🔔 Попереджати за 10 хвилин до:

ℹ️ Сповіщення надсилаються з 6:00 до 23:00`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_not_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Налаштування доступні тільки для підписаних користувачів. Спочатку підпишіться на оновлення.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_settings",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_is_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.SettingsAlerts(tt.args.c(ctrl)), "SettingsAlerts(_)")
		})
	}
}

func TestHandler_SettingsNotificationsFormat(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_subscribed_with_linear_format",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinear,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_subscribed_with_grouped_format",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatGrouped,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_not_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Налаштування доступні тільки для підписаних користувачів. Спочатку підпішіться на оновлення.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_settings",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_is_subscribed",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(false, assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.SettingsNotificationsFormat(tt.args.c(ctrl)), "SettingsNotificationsFormat(_)")
		})
	}
}

func TestHandler_SetFormatHandler(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		format string
		ctx    func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_set_linear",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinear).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinear,
					}, nil)
					return res
				},
			},
			args: args{
				format: dal.ShutdownsMessageFormatLinear,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_set_linear_with_range",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinearWithRange).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinearWithRange,
					}, nil)
					return res
				},
			},
			args: args{
				format: dal.ShutdownsMessageFormatLinearWithRange,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_set_grouped",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatGrouped).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatGrouped,
					}, nil)
					return res
				},
			},
			args: args{
				format: dal.ShutdownsMessageFormatGrouped,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_set_setting",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinear).Return(assert.AnError)
					return res
				},
			},
			args: args{
				format: dal.ShutdownsMessageFormatLinear,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_settings",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinear).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(nil, assert.AnError)
					return res
				},
			},
			args: args{
				format: dal.ShutdownsMessageFormatLinear,
				ctx: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), nil, nil, 12, 0, slog.New(slog.DiscardHandler))
			fn := h.SetFormatHandler(tt.args.format)
			tt.wantErr(t, fn(tt.args.ctx(ctrl)), "SetFormatHandler(_)")
		})
	}
}

func TestHandler_Callback(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_subscribe",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"2"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "subscribe",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_manage_groups",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"2"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "subscribe",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_unsubscribe",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().Unsubscribe(chatID).Return(nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "\funsubscribe",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Ви відписані`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_settings",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "settings",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`⚙️ Налаштування

Оберіть розділ налаштувань:`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_notify_off",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyOff, true).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(nil, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "toggle_notify_off",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`🔔 Попереджати за 10 хвилин до:

ℹ️ Сповіщення надсилаються з 6:00 до 23:00`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_notify_on",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyOn, true).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(nil, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "toggle_notify_on",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`🔔 Попереджати за 10 хвилин до:

ℹ️ Сповіщення надсилаються з 6:00 до 23:00`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_notify_maybe",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleSetting(chatID, dal.SettingNotifyMaybe, true).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(nil, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "toggle_notify_maybe",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`🔔 Попереджати за 10 хвилин до:

ℹ️ Сповіщення надсилаються з 6:00 до 23:00`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_back_from_settings",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"1"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "back_from_settings",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Привіт! Ви підписані на групи: 1`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_back",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"1"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "back",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Привіт! Ви підписані на групи: 1`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_group_5",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().ToggleGroupSubscription(chatID, "5").Return(nil)
					res.EXPECT().GetSubscribedGroups(chatID).Return([]string{"1", "5"}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "toggle_group_5",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`✅ Підписано на групу 5

Оберіть групи для підписки
(натисніть щоб додати/видалити)`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_settings_alerts",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingNotifyOn:    true,
						dal.SettingNotifyMaybe: false,
						dal.SettingNotifyOff:   false,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "settings_alerts",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_settings_notifications_format",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinear,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "settings_notifications_format",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_set_format_linear",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinear).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinear,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "set_format_linear",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_set_format_linear_with_range",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatLinearWithRange).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatLinearWithRange,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "set_format_linear_with_range",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_set_format_grouped",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().SetSetting(chatID, dal.SettingShutdownsMessageFormat, dal.ShutdownsMessageFormatGrouped).Return(nil)
					res.EXPECT().GetSettings(chatID).Return(map[dal.SettingKey]any{
						dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatGrouped,
					}, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "set_format_grouped",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_back_from_alerts",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "back_from_alerts",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_back_from_format",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().IsSubscribed(chatID).Return(true, nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "back_from_format",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(gomock.Any(), gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_unknown_data",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(nil)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "unknown",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_respond",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Respond().Return(assert.AnError)
					res.EXPECT().Callback().Return(&tb.Callback{
						Data: "unknown",
					}).AnyTimes()
					res.EXPECT().Delete().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_nil_callback",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil).AnyTimes()
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(
				tt.fields.subscriptions(ctrl),
				nil,
				nil,
				12,
				0,
				slog.New(slog.DiscardHandler),
			)
			tt.wantErr(t, h.Callback(tt.args.c(ctrl)), "Callback(_)")
		})
	}
}

func TestHandler_Unsubscribe(t *testing.T) {
	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		c func(*gomock.Controller) tb.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().Unsubscribe(chatID).Return(nil)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Ви відписані`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_unsubscribe",
			fields: fields{
				subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(ctrl)
					res.EXPECT().Unsubscribe(gomock.Any()).Return(assert.AnError)
					return res
				},
			},
			args: args{
				c: func(ctrl *gomock.Controller) tb.Context {
					res := mocks.NewMockTelebotContext(ctrl)
					res.EXPECT().Sender().Return(defaultUser).AnyTimes()
					res.EXPECT().Callback().Return(nil)
					res.EXPECT().Send(`Щось пішло не так. Будь ласка, спробуйте пізніше.`, gomock.Not(gomock.Nil())).Return(nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			h := telegram.NewHandler(
				tt.fields.subscriptions(ctrl),
				nil,
				nil,
				12,
				0,
				slog.New(slog.DiscardHandler),
			)
			tt.wantErr(t, h.Unsubscribe(tt.args.c(ctrl)), "Unsubscribe(_)")
		})
	}
}

func TestHandler_Status(t *testing.T) {
	const ownerID = int64(999)

	type args struct {
		ownerChatID int64
		sender      *tb.User
	}
	tests := []struct {
		name          string
		args          args
		subscriptions func(*gomock.Controller) telegram.Subscriptions
		ctx           func(*mocks.MockTelebotContext)
		wantErr       assert.ErrorAssertionFunc
	}{
		{
			name: "owner gets count",
			args: args{ownerChatID: ownerID, sender: &tb.User{ID: ownerID}},
			subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
				m := mocks.NewMockSubscriptions(ctrl)
				m.EXPECT().CountSubscriptions().Return(42, nil)
				return m
			},
			ctx: func(c *mocks.MockTelebotContext) {
				c.EXPECT().Callback().Return(nil)
				c.EXPECT().Send("Всього підписників: 42", gomock.Nil()).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "count error sends generic message",
			args: args{ownerChatID: ownerID, sender: &tb.User{ID: ownerID}},
			subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
				m := mocks.NewMockSubscriptions(ctrl)
				m.EXPECT().CountSubscriptions().Return(0, assert.AnError)
				return m
			},
			ctx: func(c *mocks.MockTelebotContext) {
				c.EXPECT().Callback().Return(nil)
				c.EXPECT().Send("Щось пішло не так. Будь ласка, спробуйте пізніше.", gomock.Nil()).Return(nil)
			},
			wantErr: assert.NoError,
		},
		{
			name: "non-owner ignored",
			args: args{ownerChatID: ownerID, sender: defaultUser},
			subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
				return mocks.NewMockSubscriptions(ctrl)
			},
			ctx:     func(*mocks.MockTelebotContext) {},
			wantErr: assert.NoError,
		},
		{
			name: "disabled when owner not configured",
			args: args{ownerChatID: 0, sender: &tb.User{ID: 0}},
			subscriptions: func(ctrl *gomock.Controller) telegram.Subscriptions {
				return mocks.NewMockSubscriptions(ctrl)
			},
			ctx:     func(*mocks.MockTelebotContext) {},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			c := mocks.NewMockTelebotContext(ctrl)
			c.EXPECT().Sender().Return(tt.args.sender).AnyTimes()
			tt.ctx(c)

			h := telegram.NewHandler(tt.subscriptions(ctrl), nil, nil, 12, tt.args.ownerChatID, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.Status(c), "Status")
		})
	}
}
