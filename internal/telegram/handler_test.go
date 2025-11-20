package telegram_test

import (
	"log/slog"
	"testing"

	"github.com/Roma7-7-7/sso-notifier/internal/telegram/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
	tb "gopkg.in/telebot.v3"

	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
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

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), 12, slog.New(slog.DiscardHandler))
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

			h := telegram.NewHandler(tt.fields.subscriptions(ctrl), 12, slog.New(slog.DiscardHandler))
			tt.wantErr(t, h.ManageGroups(tt.args.c(ctrl)), "ManageGroups(_)")
		})
	}
}
