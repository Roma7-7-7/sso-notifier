package service

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

type SubscriptionsStore interface {
	ExistsSubscription(chatID int64) (bool, error)
	GetAllSubscriptions() ([]dal.Subscription, error)
	GetSubscription(chatID int64) (dal.Subscription, bool, error)
	PutSubscription(sub dal.Subscription) error
	PurgeSubscriptions(chatID int64) error
}

type Subscriptions struct {
	store SubscriptionsStore

	log *slog.Logger
	mx  *sync.Mutex
}

func NewSubscription(store SubscriptionsStore, log *slog.Logger) *Subscriptions {
	return &Subscriptions{
		store: store,
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

func (s *Subscriptions) SubscribeToGroup(chatID int64, groupNum string) error {
	sub, exists, err := s.store.GetSubscription(chatID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	if !exists {
		sub = dal.Subscription{
			ChatID:    chatID,
			CreatedAt: time.Now(),
		}
	}

	sub.Groups = map[string]string{
		groupNum: "",
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
	return s.store.PurgeSubscriptions(chatID)
}
