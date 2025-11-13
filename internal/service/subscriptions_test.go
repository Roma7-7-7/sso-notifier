package service_test

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/mocks"
)

const chatID = int64(123)

func TestSubscriptions_IsSubscribed(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().ExistsSubscription(chatID).Return(true, nil)

		exists, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).IsSubscribed(chatID)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().ExistsSubscription(chatID).Return(false, assert.AnError)

		_, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).IsSubscribed(chatID)
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "check if subscription exists: ")
	})
}

func TestSubscriptions_GetSubscriptions(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{
			testutil.NewSubscription(123).Build(),
			testutil.NewSubscription(456).Build(),
			testutil.NewSubscription(789).Build(),
		}, nil)

		subs, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscriptions()
		require.NoError(t, err)
		if assert.Len(t, subs, 3) {
			assert.Equal(t, subs, []dal.Subscription{
				testutil.NewSubscription(123).WithCreatedAt(subs[0].CreatedAt).Build(),
				testutil.NewSubscription(456).WithCreatedAt(subs[1].CreatedAt).Build(),
				testutil.NewSubscription(789).WithCreatedAt(subs[2].CreatedAt).Build(),
			})
		}
	})

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetAllSubscriptions().Return([]dal.Subscription{}, assert.AnError)
		_, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscriptions()
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "get subscriptions: ")
	})
}

func TestSubscriptions_GetSubscribedGroups(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).Return(
			testutil.NewSubscription(chatID).WithGroups("1", "5", "11").Build(),
			true,
			nil,
		)

		groups, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscribedGroups(chatID)
		require.NoError(t, err)
		if assert.Len(t, groups, 3) {
			assert.ElementsMatch(t, []string{"1", "5", "11"}, groups)
		}
	})

	t.Run("not_exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).Return(
			dal.Subscription{},
			false,
			nil,
		)

		groups, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscribedGroups(chatID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("exists_but_groups_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		sub := testutil.NewSubscription(chatID).Build()
		sub.Groups = nil
		store.EXPECT().GetSubscription(chatID).Return(
			sub,
			true,
			nil,
		)

		groups, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscribedGroups(chatID)
		require.NoError(t, err)
		assert.Empty(t, groups)
	})

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).Return(
			dal.Subscription{},
			false,
			assert.AnError,
		)

		_, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSubscribedGroups(chatID)
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "get subscription: ")
	})
}

func TestSubscriptions_ToggleGroupSubscription(t *testing.T) {
	type fields struct {
		store func(*testing.T, *gomock.Controller) service.SubscriptionsStore
	}
	type args struct {
		chatID   int64
		groupNum string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "ok_user_not_subscribed",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(dal.Subscription{}, false, nil)
					store.EXPECT().
						PutSubscription(testutil.NewSubscription(chatID).WithGroups("4").BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "4",
			},
			wantErr: assert.NoError,
		},
		{
			name: "ok_user_subscribed_but_has_no_groups",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(testutil.NewSubscription(chatID).Build(), true, nil)
					store.EXPECT().
						PutSubscription(testutil.NewSubscription(chatID).WithGroups("4").BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "4",
			},
			wantErr: assert.NoError,
		},
		{
			name: "ok_user_subscribed_to_other_group",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(testutil.NewSubscription(chatID).WithGroups("2").Build(), true, nil)
					store.EXPECT().
						PutSubscription(testutil.NewSubscription(chatID).WithGroups("2", "4").BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "4",
			},
			wantErr: assert.NoError,
		},
		{
			name: "ok_user_subscribed_to_group",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(testutil.NewSubscription(chatID).WithGroups("2", "4", "6").Build(), true, nil)
					store.EXPECT().
						PutSubscription(testutil.NewSubscription(chatID).WithGroups("2", "6").BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "4",
			},
			wantErr: assert.NoError,
		},
		{
			name: "ok_user_unsubscribed_from_last_group",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(testutil.NewSubscription(chatID).WithGroups("7").Build(), true, nil)
					store.EXPECT().Purge(chatID)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "7",
			},
			wantErr: assert.NoError,
		},
		{
			name: "ok_if_subscription_has_nil_group",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					sub := testutil.NewSubscription(chatID).Build()
					sub.Groups = nil
					store.EXPECT().GetSubscription(chatID).Return(sub, true, nil)
					store.EXPECT().PutSubscription(testutil.NewSubscription(chatID).WithGroups("7").BuildMatcher(t)).Return(nil)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "7",
			},
			wantErr: assert.NoError,
		},
		{
			name: "error_put_subscription",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(testutil.NewSubscription(chatID).Build(), true, nil)
					store.EXPECT().PutSubscription(gomock.Any()).Return(assert.AnError)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "7",
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "put subscription: "),
		},
		{
			name: "error_get_subscription",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().GetSubscription(chatID).Return(dal.Subscription{}, false, assert.AnError)
					return store
				},
			},
			args: args{
				chatID:   chatID,
				groupNum: "7",
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get subscription: "),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := service.NewSubscription(tt.fields.store(t, ctrl), slog.New(slog.DiscardHandler))
			tt.wantErr(t, s.ToggleGroupSubscription(tt.args.chatID, tt.args.groupNum), fmt.Sprintf("ToggleGroupSubscription(%v, %v)", tt.args.chatID, tt.args.groupNum))
		})
	}
}

func TestSubscriptions_Unsubscribe(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().Purge(chatID).Return(nil)

		require.NoError(t, service.NewSubscription(store, slog.New(slog.DiscardHandler)).Unsubscribe(chatID))
	})

	t.Run("error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().Purge(chatID).Return(assert.AnError)

		err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).Unsubscribe(chatID)
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "purge subscription: ")
	})
}

func TestSubscriptions_GetSettings(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).
			Return(
				testutil.NewSubscription(chatID).WithSetting(dal.SettingNotifyOn, true).WithSetting(dal.SettingNotifyMaybe, false).Build(),
				true,
				nil,
			)

		settings, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSettings(chatID)
		require.NoError(t, err)
		if assert.Len(t, settings, 2) {
			assert.Equal(t, true, settings[dal.SettingNotifyOn], "settings should have been set")
			assert.Equal(t, false, settings[dal.SettingNotifyMaybe], "settings should not be set")
		}
	})

	t.Run("not_exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).Return(dal.Subscription{}, false, nil)

		settings, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSettings(chatID)
		require.NoError(t, err)
		assert.Empty(t, settings)
	})

	t.Run("exists_but_settings_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		sub := testutil.NewSubscription(chatID).Build()
		sub.Settings = nil
		store.EXPECT().GetSubscription(chatID).Return(sub, true, nil)

		settings, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSettings(chatID)
		require.NoError(t, err)
		assert.Empty(t, settings)
	})

	t.Run("error_get_subscription", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mocks.NewMockSubscriptionsStore(ctrl)
		store.EXPECT().GetSubscription(chatID).Return(dal.Subscription{}, false, assert.AnError)

		_, err := service.NewSubscription(store, slog.New(slog.DiscardHandler)).GetSettings(chatID)
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "get subscription: ")
	})
}

func TestSubscriptions_ToggleSetting(t *testing.T) {
	type fields struct {
		store func(*testing.T, *gomock.Controller) service.SubscriptionsStore
	}
	type args struct {
		chatID       int64
		key          dal.SettingKey
		defaultValue bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "success_toggle_setting_not_present_default_true",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_setting_not_present_default_false",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, false).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: false,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_setting_on_default_false",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, false).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: false,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_setting_on_default_true",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, false).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_setting_off_default_false",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, false).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: false,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_toggle_setting_off_default_true",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, false).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_if_settings_is_nil_default_false",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					sub := testutil.NewSubscription(chatID).Build()
					sub.Settings = nil
					store.EXPECT().
						GetSubscription(chatID).
						Return(sub, true, nil)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyMaybe, false).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyMaybe,
				defaultValue: false,
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_if_settings_is_nil_default_true",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					sub := testutil.NewSubscription(chatID).Build()
					sub.Settings = nil
					store.EXPECT().
						GetSubscription(chatID).
						Return(sub, true, nil)
					store.EXPECT().
						PutSubscription(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyMaybe, true).
							BuildMatcher(t)).
						Return(nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyMaybe,
				defaultValue: true,
			},
			wantErr: assert.NoError,
		},
		{
			name: "subscription_not_exists",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(dal.Subscription{}, false, nil)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: testutil.AssertErrorIsAndContains(service.ErrSubscriptionNotFound, "subscription for chatID 123: "),
		},
		{
			name: "error_put_subscription",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(testutil.
							NewSubscription(chatID).
							WithSetting(dal.SettingNotifyOn, true).
							WithSetting(dal.SettingNotifyOff, true).
							Build(),
							true,
							nil,
						)
					store.EXPECT().
						PutSubscription(gomock.Any()).
						Return(assert.AnError)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "put subscription: "),
		},
		{
			name: "error_get_subscription",
			fields: fields{
				store: func(t *testing.T, ctrl *gomock.Controller) service.SubscriptionsStore {
					t.Helper()
					store := mocks.NewMockSubscriptionsStore(ctrl)
					store.EXPECT().
						GetSubscription(chatID).
						Return(dal.Subscription{}, false, assert.AnError)
					return store
				},
			},
			args: args{
				chatID:       chatID,
				key:          dal.SettingNotifyOff,
				defaultValue: true,
			},
			wantErr: testutil.AssertErrorIsAndContains(assert.AnError, "get subscription: "),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			s := service.NewSubscription(tt.fields.store(t, ctrl), slog.New(slog.DiscardHandler))
			tt.wantErr(t, s.ToggleSetting(tt.args.chatID, tt.args.key, tt.args.defaultValue), fmt.Sprintf("ToggleSetting(%v, %v, %v)", tt.args.chatID, tt.args.key, tt.args.defaultValue))
		})
	}
}
