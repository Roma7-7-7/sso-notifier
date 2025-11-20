package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
)

//go:generate mockgen -package mocks -destination mocks/shutdowns.go . ShutdownsStore,ShutdownsProvider

type ShutdownsStore interface {
	GetShutdowns(d dal.Date) (dal.Shutdowns, bool, error)
	PutShutdowns(d dal.Date, s dal.Shutdowns) error
}

type ShutdownsProvider interface {
	Shutdowns(ctx context.Context) (dal.Shutdowns, bool, error)
	ShutdownsNext(ctx context.Context) (dal.Shutdowns, error)
}

type Shutdowns struct {
	store    ShutdownsStore
	provider ShutdownsProvider
	clock    Clock

	loc *time.Location
	log *slog.Logger
	mx  *sync.Mutex
}

func NewShutdowns(store ShutdownsStore, provider ShutdownsProvider, clock Clock, log *slog.Logger) *Shutdowns {
	return &Shutdowns{
		store:    store,
		provider: provider,
		clock:    clock,
		log:      log.With("component", "service").With("service", "shutdowns"),
		mx:       &sync.Mutex{},
	}
}

func (s *Shutdowns) Refresh(ctx context.Context) error {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.log.InfoContext(ctx, "refreshing shutdowns")

	ctx, cancelFunc := context.WithTimeout(ctx, time.Minute)
	defer cancelFunc()

	now := s.clock.Now()
	today := dal.DateByTime(now)
	todayTable, nextDayAvailable, err := s.provider.Shutdowns(ctx)
	if err != nil {
		if !errors.Is(err, providers.ErrCheckNextDayAvailability) {
			return fmt.Errorf("get shutdowns for today: %w", err)
		}
		s.log.WarnContext(ctx, "failed to check next day availability", "error", err)
	}

	if err = s.store.PutShutdowns(today, todayTable); err != nil {
		return fmt.Errorf("put shutdowns for today: %w", err)
	}
	s.log.InfoContext(ctx, "refreshed today's shutdowns", "date", today.ToKey())

	// Fetch tomorrow's schedule only if it's available
	if !nextDayAvailable {
		s.log.InfoContext(ctx, "tomorrow's shutdowns not yet available")
		return nil
	}

	tomorrow := dal.DateByTime(now.AddDate(0, 0, 1))
	tomorrowTable, err := s.provider.ShutdownsNext(ctx)
	if err != nil {
		// Log error but don't fail the entire refresh - today's data is already saved
		s.log.ErrorContext(ctx, "failed to get tomorrow's shutdowns", "error", err)
		return nil
	}

	if err = s.store.PutShutdowns(tomorrow, tomorrowTable); err != nil {
		// Log error but don't fail - today's data is already saved
		s.log.ErrorContext(ctx, "failed to put tomorrow's shutdowns", "error", err)
		return nil
	}
	s.log.InfoContext(ctx, "refreshed tomorrow's shutdowns", "date", tomorrow.ToKey())

	return nil
}
