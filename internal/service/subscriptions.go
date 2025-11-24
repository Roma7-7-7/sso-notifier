package service

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

//go:generate mockgen -package mocks -destination mocks/subscriptions.go . SubscriptionsStore

var ErrSubscriptionNotFound = errors.New("subscription not found")

type SubscriptionsStore interface {
	ExistsSubscription(chatID int64) (bool, error)
	GetAllSubscriptions() ([]dal.Subscription, error)
	GetSubscription(chatID int64) (dal.Subscription, bool, error)
	PutSubscription(sub dal.Subscription) error
	Purge(chatID int64) error
}

type Subscriptions struct {
	store SubscriptionsStore
	clock Clock

	log *slog.Logger
	mx  *sync.Mutex
}

func NewSubscription(store SubscriptionsStore, clock Clock, log *slog.Logger) *Subscriptions {
	return &Subscriptions{
		store: store,
		clock: clock,
		log:   log.With("component", "service").With("service", "subscriptions"),
		mx:    &sync.Mutex{},
	}
}

func (s *Subscriptions) IsSubscribed(chatID int64) (bool, error) {
	exists, err := s.store.ExistsSubscription(chatID)
	if err != nil {
		return false, fmt.Errorf("check if subscription exists: %w", err)
	}
	return exists, nil
}

func (s *Subscriptions) GetSubscriptions() ([]dal.Subscription, error) {
	subs, err := s.store.GetAllSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("get subscriptions: %w", err)
	}
	return subs, nil
}

func (s *Subscriptions) GetSubscribedGroups(chatID int64) ([]string, error) {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	if !exists {
		return []string{}, nil
	}

	groups := make([]string, 0, len(sub.Groups))
	for groupNum := range sub.Groups {
		groups = append(groups, groupNum)
	}

	return groups, nil
}

func (s *Subscriptions) ToggleGroupSubscription(chatID int64, groupNum string) error {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if !exists {
		sub = dal.Subscription{
			ChatID:    chatID,
			CreatedAt: s.clock.Now(),
			Groups:    make(map[string]struct{}),
		}
	}

	if sub.Groups == nil {
		sub.Groups = make(map[string]struct{})
	}

	// Toggle: if exists, remove; if not, add
	if _, subscribed := sub.Groups[groupNum]; subscribed {
		delete(sub.Groups, groupNum)
		s.log.Debug("unsubscribed from group", "chatID", chatID, "groupNum", groupNum)
	} else {
		sub.Groups[groupNum] = struct{}{}
		s.log.Debug("subscribed to group", "chatID", chatID, "groupNum", groupNum)
	}

	// If no groups left, delete the entire subscription
	if len(sub.Groups) == 0 {
		if exists {
			return s.Unsubscribe(chatID)
		}
		return nil
	}

	err = s.store.PutSubscription(sub)
	if err != nil {
		return fmt.Errorf("put subscription: %w", err)
	}

	if !exists {
		s.log.Debug("new subscriber", "chatID", chatID)
	}

	return nil
}

func (s *Subscriptions) Unsubscribe(chatID int64) error {
	if err := s.store.Purge(chatID); err != nil {
		return fmt.Errorf("purge subscription: %w", err)
	}
	return nil
}

// GetSettings retrieves the settings map for a user
func (s *Subscriptions) GetSettings(chatID int64) (map[dal.SettingKey]interface{}, error) {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	if !exists || sub.Settings == nil {
		return make(map[dal.SettingKey]interface{}), nil
	}

	return sub.Settings, nil
}

// ToggleSetting toggles a boolean setting for a user
func (s *Subscriptions) ToggleSetting(chatID int64, key dal.SettingKey, defaultValue bool) error {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if !exists {
		return fmt.Errorf("subscription for chatID %d: %w", chatID, ErrSubscriptionNotFound)
	}

	if sub.Settings == nil {
		sub.Settings = make(map[dal.SettingKey]interface{})
	}

	// use !defaultValue because we inverse it below with newValue := !currentValue
	currentValue := dal.GetBoolSetting(sub.Settings, key, !defaultValue)

	newValue := !currentValue
	sub.Settings[key] = newValue

	if err := s.store.PutSubscription(sub); err != nil {
		return fmt.Errorf("put subscription: %w", err)
	}

	s.log.Debug("toggled setting",
		"chatID", chatID,
		"key", key,
		"oldValue", currentValue,
		"newValue", newValue)

	return nil
}

// SetSetting sets a setting value for a user
func (s *Subscriptions) SetSetting(chatID int64, key dal.SettingKey, value interface{}) error {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if !exists {
		return fmt.Errorf("subscription for chatID %d: %w", chatID, ErrSubscriptionNotFound)
	}

	if sub.Settings == nil {
		sub.Settings = make(map[dal.SettingKey]interface{})
	}

	sub.Settings[key] = value

	if err := s.store.PutSubscription(sub); err != nil {
		return fmt.Errorf("put subscription: %w", err)
	}

	s.log.Debug("set setting",
		"chatID", chatID,
		"key", key,
		"value", value)

	return nil
}
