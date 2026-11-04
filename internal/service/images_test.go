package service_test

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/internal/render"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func TestScheduleImage(t *testing.T) {
	t.Run("subtitle and segments", func(t *testing.T) {
		img := service.ScheduleImage{
			Date:     "5 листопада",
			GroupNum: "4",
			Periods: []dal.Period{
				{From: "00:00", To: "01:00"}, {From: "01:00", To: "03:00"}, {From: "03:00", To: "03:30"},
				{From: "03:30", To: "18:00"}, {From: "18:00", To: "21:00"}, {From: "21:00", To: "21:30"}, {From: "21:30", To: "24:00"},
			},
			Statuses: []dal.Status{dal.ON, dal.OFF, dal.MAYBE, dal.ON, dal.OFF, dal.MAYBE, dal.ON},
		}

		day, err := img.Day()
		require.NoError(t, err)
		assert.Equal(t, "5 листопада: Група 4", day.Title)
		assert.Equal(t, "без світла 5г · можливо 1г", day.Subtitle)
		assert.Equal(t, render.Segment{FromMin: 1290, ToMin: 1440, Kind: render.KindOn}, day.Segments[len(day.Segments)-1])
	})

	t.Run("windows of any length", func(t *testing.T) {
		img := service.ScheduleImage{
			Date:     "5 листопада",
			GroupNum: "1",
			Periods: []dal.Period{
				{From: "00:00", To: "01:00"}, {From: "01:00", To: "01:15"}, {From: "01:15", To: "02:30"}, {From: "02:30", To: "03:45"}, {From: "03:45", To: "24:00"},
			},
			Statuses: []dal.Status{dal.ON, dal.MAYBE, dal.OFF, dal.MAYBE, dal.ON},
		}

		day, err := img.Day()
		require.NoError(t, err)
		assert.Equal(t, "без світла 1г 15хв · можливо 1г 30хв", day.Subtitle)
		assert.Equal(t, render.Segment{FromMin: 60, ToMin: 75, Kind: render.KindMaybe}, day.Segments[1])
	})

	t.Run("invalid data", func(t *testing.T) {
		tests := map[string]service.ScheduleImage{
			"bad time":       {Periods: []dal.Period{{From: "xx", To: "01:00"}}, Statuses: []dal.Status{dal.ON}},
			"unknown status": {Periods: []dal.Period{{From: "00:00", To: "01:00"}}, Statuses: []dal.Status{"?"}},
		}
		for name, img := range tests {
			t.Run(name, func(t *testing.T) {
				_, err := img.Day()
				require.Error(t, err)
			})
		}
	})
}

type fakePhotos struct {
	err    error
	sent   []string // chatID
	photos [][]byte
}

func (f *fakePhotos) SendPhoto(_ context.Context, chatID string, photo []byte) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, chatID)
	f.photos = append(f.photos, photo)
	return nil
}

func TestNotifications_NotifyPowerSupplySchedule_Image(t *testing.T) {
	now := time.Date(2025, time.November, 20, 10, 0, 0, 0, time.UTC)
	today := dal.Date{Year: 2025, Month: time.November, Day: 20}
	tomorrow := dal.Date{Year: 2025, Month: time.November, Day: 21}

	shutdowns := testutil.NewShutdowns().
		WithDate("20 листопада").
		WithGroup(1, "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
		WithGroup(3, "YYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMN").
		Build()

	const chatID = int64(123)
	chatIDStr := strconv.FormatInt(chatID, 10)
	sub := testutil.NewSubscription(chatID).WithGroups("1", "3").Build()
	sub.Settings = map[dal.SettingKey]any{dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatImage}

	build := func(ctrl *gomock.Controller, telegram service.TelegramClient, images bool, r service.ImageRenderer, p service.PhotoSender) *service.Notifications {
		sh := mocks.NewMockShutdownsStore(ctrl)
		sh.EXPECT().GetShutdowns(today).Return(shutdowns, true, nil)
		sh.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
		subs := mocks.NewMockSubscriptionsStore(ctrl)
		subs.EXPECT().GetSubscription(chatID).Return(sub, true, nil)

		n := service.NewNotifications(sh, subs, mocks.NewMockNotificationsStore(ctrl), telegram, clock.NewMock(now), time.Hour, slog.New(slog.DiscardHandler))
		if images {
			n = n.WithImages(r, p)
		}
		return n
	}
	okRender := func(d render.Day) ([]byte, error) { return []byte("png:" + d.Title), nil }

	expectFallbackText := func(ctrl *gomock.Controller) *mocks.MockTelegramClient {
		tg := mocks.NewMockTelegramClient(ctrl)
		tg.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).DoAndReturn(func(_ context.Context, _, text string) error {
			assert.True(t, strings.HasPrefix(text, "Графік стабілізаційних відключень:"), text)
			assert.Contains(t, text, "Група 1:")
			assert.Contains(t, text, "Група 3:")
			assert.Contains(t, text, "🔴 10:30 - 13:30", "fallback keeps ranges so it is as informative as the image")
			return nil
		})
		return tg
	}

	t.Run("sends one photo per group and no text", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		photos := &fakePhotos{}
		n := build(ctrl, mocks.NewMockTelegramClient(ctrl), true, okRender, photos)

		require.NoError(t, n.NotifyPowerSupplySchedule(t.Context(), chatID))

		assert.Equal(t, []string{chatIDStr, chatIDStr}, photos.sent)
		assert.Equal(t, [][]byte{[]byte("png:20 листопада: Група 1"), []byte("png:20 листопада: Група 3")}, photos.photos)
	})

	t.Run("render failure falls back to text and sends no photos", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		photos := &fakePhotos{}
		failing := func(render.Day) ([]byte, error) { return nil, errors.New("boom") }
		n := build(ctrl, expectFallbackText(ctrl), true, failing, photos)

		require.NoError(t, n.NotifyPowerSupplySchedule(t.Context(), chatID))
		assert.Empty(t, photos.sent)
	})

	t.Run("upload failure falls back to text", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		photos := &fakePhotos{err: errors.New("telegram is down")}
		n := build(ctrl, expectFallbackText(ctrl), true, okRender, photos)

		require.NoError(t, n.NotifyPowerSupplySchedule(t.Context(), chatID))
	})

	t.Run("images not configured falls back to text", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		n := build(ctrl, expectFallbackText(ctrl), false, nil, nil)

		require.NoError(t, n.NotifyPowerSupplySchedule(t.Context(), chatID))
	})

	t.Run("fallback text failure is reported", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tg := mocks.NewMockTelegramClient(ctrl)
		sendErr := errors.New("send failed")
		tg.EXPECT().SendMessage(gomock.Any(), chatIDStr, gomock.Any()).Return(sendErr)
		n := build(ctrl, tg, true, okRender, &fakePhotos{err: errors.New("down")})

		require.ErrorIs(t, n.NotifyPowerSupplySchedule(t.Context(), chatID), sendErr)
	})
}

func TestNotifications_NotifyPowerSupplyScheduleUpdates_Image(t *testing.T) {
	now := time.Date(2025, time.November, 20, 10, 0, 0, 0, time.UTC)
	today := dal.Date{Year: 2025, Month: time.November, Day: 20}
	tomorrow := dal.Date{Year: 2025, Month: time.November, Day: 21}

	shutdowns := testutil.NewShutdowns().
		WithDate("20 листопада").
		WithGroup(1, "YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY").
		Build()

	const chatID = int64(123)
	sub := testutil.NewSubscription(chatID).WithGroups("1").Build()
	sub.Settings = map[dal.SettingKey]any{dal.SettingShutdownsMessageFormat: dal.ShutdownsMessageFormatImage}

	run := func(t *testing.T, tg func(*gomock.Controller) service.TelegramClient, photos *fakePhotos, expectSaved bool) {
		ctrl := gomock.NewController(t)
		sh := mocks.NewMockShutdownsStore(ctrl)
		sh.EXPECT().GetEmergencyState().Return(dal.EmergencyState{}, nil)
		sh.EXPECT().GetShutdowns(today).Return(shutdowns, true, nil)
		sh.EXPECT().GetShutdowns(tomorrow).Return(dal.Shutdowns{}, false, nil)
		subs := mocks.NewMockSubscriptionsStore(ctrl)
		subs.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{sub}, nil)
		store := mocks.NewMockNotificationsStore(ctrl)
		store.EXPECT().GetNotificationState(chatID, today).Return(dal.NotificationState{}, false, nil)
		store.EXPECT().GetNotificationState(chatID, tomorrow).Return(dal.NotificationState{}, false, nil)
		if expectSaved {
			store.EXPECT().PutNotificationState(gomock.Any()).DoAndReturn(func(s dal.NotificationState) error {
				assert.NotEmpty(t, s.Hashes["1"], "hash must be stored so the same schedule is not sent again")
				return nil
			})
		}

		n := service.NewNotifications(sh, subs, store, tg(ctrl), clock.NewMock(now), time.Hour, slog.New(slog.DiscardHandler)).
			WithImages(func(d render.Day) ([]byte, error) { return []byte("png"), nil }, photos)
		require.NoError(t, n.NotifyPowerSupplyScheduleUpdates(t.Context()))
	}

	t.Run("photo delivered: state saved, no text", func(t *testing.T) {
		photos := &fakePhotos{}
		run(t, func(c *gomock.Controller) service.TelegramClient { return mocks.NewMockTelegramClient(c) }, photos, true)
		assert.Len(t, photos.sent, 1)
	})

	t.Run("photo failed, text delivered: state saved", func(t *testing.T) {
		run(t, func(c *gomock.Controller) service.TelegramClient {
			tg := mocks.NewMockTelegramClient(c)
			tg.EXPECT().SendMessage(gomock.Any(), "123", gomock.Any()).Return(nil)
			return tg
		}, &fakePhotos{err: errors.New("down")}, true)
	})

	t.Run("photo and text failed: state not saved so it is retried", func(t *testing.T) {
		run(t, func(c *gomock.Controller) service.TelegramClient {
			tg := mocks.NewMockTelegramClient(c)
			tg.EXPECT().SendMessage(gomock.Any(), "123", gomock.Any()).Return(errors.New("down"))
			return tg
		}, &fakePhotos{err: errors.New("down")}, false)
	})
}
