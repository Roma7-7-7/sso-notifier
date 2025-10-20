package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
)

type ShutdownsStore interface {
	GetShutdowns() (dal.Shutdowns, bool, error)
	PutShutdowns(s dal.Shutdowns) error
}

type Shutdowns struct {
	store ShutdownsStore

	log *slog.Logger
	mx  *sync.Mutex
}

func NewShutdowns(store ShutdownsStore, log *slog.Logger) *Shutdowns {
	return &Shutdowns{
		store: store,
		log:   log.With("service", "shutdowns"),
		mx:    &sync.Mutex{},
	}
}

func (s *Shutdowns) Refresh(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "refreshing shutdowns")

	ctx, cancelFunc := context.WithTimeout(ctx, time.Minute)
	defer cancelFunc()

	table, err := providers.ChernivtsiShutdowns(ctx)
	if err != nil {
		return fmt.Errorf("get chernivtsi shutdowns: %w", err)
	}
	if err = s.store.PutShutdowns(table); err != nil {
		return fmt.Errorf("put shutdowns: %w", err)
	}

	return nil
}
