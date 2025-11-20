package service_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func TestShutdowns_Refresh(t *testing.T) {
	now := time.Now().UTC()
	todayDate := dal.Date{Year: now.Year(), Month: now.Month(), Day: now.Day()}
	tomorrowDate := dal.Date{Year: now.Year(), Month: now.Month(), Day: now.Day() + 1}
	defaultTodayShutdowns := testutil.NewShutdowns().WithDate(now.Format(time.DateOnly)).Build()
	defaultTomorrowShutdowns := testutil.NewShutdowns().WithDate(now.AddDate(0, 0, 1).Format(time.DateOnly)).Build()

	type fields struct {
		store    func(*gomock.Controller) service.ShutdownsStore
		provider func(*gomock.Controller) service.ShutdownsProvider
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_without_next_day",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(nil)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, false, nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_with_next_day",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(nil)
					res.EXPECT().PutShutdowns(tomorrowDate, defaultTomorrowShutdowns).Return(nil)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, true, nil)
					res.EXPECT().ShutdownsNext(gomock.Any()).Return(defaultTomorrowShutdowns, nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_next_shutdown",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(nil)
					res.EXPECT().PutShutdowns(tomorrowDate, defaultTomorrowShutdowns).Return(assert.AnError)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, true, nil)
					res.EXPECT().ShutdownsNext(gomock.Any()).Return(defaultTomorrowShutdowns, nil)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_get_next_shutdowns",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(nil)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, true, nil)
					res.EXPECT().ShutdownsNext(gomock.Any()).Return(defaultTomorrowShutdowns, assert.AnError)
					return res
				},
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_shutdowns",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(assert.AnError)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, true, nil)
					return res
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Error(t, err, i...) && assert.ErrorIs(t, err, assert.AnError) && assert.ErrorContains(t, err, "put shutdowns for today: ")
			},
		},
		{
			name: "error_get_shutdowns",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(dal.Shutdowns{}, true, assert.AnError)
					return res
				},
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.Error(t, err, i...) && assert.ErrorIs(t, err, assert.AnError) && assert.ErrorContains(t, err, "get shutdowns for today: ")
			},
		},
		{
			name: "error_check_next_day_available",
			fields: fields{
				store: func(c *gomock.Controller) service.ShutdownsStore {
					res := mocks.NewMockShutdownsStore(c)
					res.EXPECT().PutShutdowns(todayDate, defaultTodayShutdowns).Return(nil)
					return res
				},
				provider: func(c *gomock.Controller) service.ShutdownsProvider {
					res := mocks.NewMockShutdownsProvider(c)
					res.EXPECT().Shutdowns(gomock.Any()).Return(defaultTodayShutdowns, false, providers.ErrCheckNextDayAvailability)
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

			s := service.NewShutdowns(tt.fields.store(ctrl), tt.fields.provider(ctrl), clock.NewMock(now), slog.New(slog.DiscardHandler))
			tt.wantErr(t, s.Refresh(t.Context()), "Refresh(_)")
		})
	}
}
