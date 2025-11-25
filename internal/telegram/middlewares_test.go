package telegram_test

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	tc "github.com/Roma7-7-7/telegram"
	
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram/mocks"
)

type tcContext struct {
	context.Context
	chatID string
}

func (c *tcContext) ChatID() (string, bool) {
	return c.chatID, c.chatID != ""
}

func stubContext(chatID string) *tcContext {
	return &tcContext{
		Context: context.Background(),
		chatID:  chatID,
	}
}

func handlerStub(err error) tc.Handler {
	return func(_ tc.Context) error {
		return err
	}
}

func TestPurgeOnForbiddenMiddleware_Handle(t *testing.T) {
	const chatID = int64(123)
	var chatIDStr = strconv.FormatInt(chatID, 10)

	type fields struct {
		subscriptions func(*gomock.Controller) telegram.Subscriptions
	}
	type args struct {
		chatID string
		err    error
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "no_error",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					return res
				},
			},
			args: args{
				chatID: chatIDStr,
				err:    nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "other_error",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					return res
				},
			},
			args: args{
				chatID: chatIDStr,
				err:    assert.AnError,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, assert.AnError, err, "expected error %v, got %v", assert.AnError, err)
			},
		},
		{
			name: "forbidden_error",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					res.EXPECT().Unsubscribe(chatID).Return(nil)
					return res
				},
			},
			args: args{
				chatID: chatIDStr,
				err:    tc.ErrForbidden,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, tc.ErrForbidden, err, "expected error %v, got %v", tc.ErrForbidden, err)
			},
		},
		{
			name: "wrapped_forbidden_error",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					res.EXPECT().Unsubscribe(chatID).Return(nil)
					return res
				},
			},
			args: args{
				chatID: chatIDStr,
				err:    fmt.Errorf("wrapped: %w", tc.ErrForbidden),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, fmt.Errorf("wrapped: %w", tc.ErrForbidden), err, "expected error %v, got %v", fmt.Errorf("wrapped: %w", tc.ErrForbidden), err)
			},
		},
		{
			name: "error_unsubscribe",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					res.EXPECT().Unsubscribe(chatID).Return(assert.AnError)
					return res
				},
			},
			args: args{
				chatID: chatIDStr,
				err:    fmt.Errorf("wrapped: %w", tc.ErrForbidden),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, fmt.Errorf("wrapped: %w", tc.ErrForbidden), err, "expected error %v, got %v", fmt.Errorf("wrapped: %w", tc.ErrForbidden), err)
			},
		},
		{
			name: "error_not_int_chat_id",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					return res
				},
			},
			args: args{
				chatID: "fake",
				err:    fmt.Errorf("wrapped: %w", tc.ErrForbidden),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, fmt.Errorf("wrapped: %w", tc.ErrForbidden), err, "expected error %v, got %v", fmt.Errorf("wrapped: %w", tc.ErrForbidden), err)
			},
		},
		{
			name: "error_missing_chat_id",
			fields: fields{
				subscriptions: func(*gomock.Controller) telegram.Subscriptions {
					res := mocks.NewMockSubscriptions(gomock.NewController(t))
					return res
				},
			},
			args: args{
				chatID: "",
				err:    fmt.Errorf("wrapped: %w", tc.ErrForbidden),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Equalf(t, fmt.Errorf("wrapped: %w", tc.ErrForbidden), err, "expected error %v, got %v", fmt.Errorf("wrapped: %w", tc.ErrForbidden), err)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := telegram.NewPurgeOnForbiddenMiddleware(
				tt.fields.subscriptions(ctrl),
				slog.New(slog.DiscardHandler),
			)
			hStub := handlerStub(tt.args.err)
			tt.wantErr(t, m.Handle(hStub)(stubContext(tt.args.chatID)))
		})
	}
}
