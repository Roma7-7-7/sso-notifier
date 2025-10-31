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
	GetShutdowns(d dal.Date) (dal.Shutdowns, bool, error)
	PutShutdowns(d dal.Date, s dal.Shutdowns) error
}

type Shutdowns struct {
	store ShutdownsStore

	loc *time.Location
	log *slog.Logger
	mx  *sync.Mutex
}

func NewShutdowns(store ShutdownsStore, loc *time.Location, log *slog.Logger) *Shutdowns {
	return &Shutdowns{
		store: store,
		loc:   loc,
		log:   log.With("component", "service").With("service", "shutdowns"),
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
	if err = s.store.PutShutdowns(dal.TodayDate(s.loc), table); err != nil {
		return fmt.Errorf("put shutdowns: %w", err)
	}

	return nil
}
