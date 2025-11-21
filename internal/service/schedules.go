package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

type Clock interface {
	Now() time.Time
}

type processFn func(ctx context.Context) error

type Scheduler struct {
	conf *telegram.Config

	shutdowns     *Shutdowns
	notifications *Notifications
	alerts        *Alerts

	log *slog.Logger
}

func NewScheduler(
	conf *telegram.Config,
	shutdowns *Shutdowns,
	notifications *Notifications,
	alerts *Alerts,
	log *slog.Logger,
) *Scheduler {
	return &Scheduler{
		conf: conf,

		shutdowns:     shutdowns,
		notifications: notifications,
		alerts:        alerts,

		log: log.With("component", "scheduler"),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	wg := &sync.WaitGroup{}
	wg.Go(func() {
		s.run(ctx, s.conf.RefreshShutdownsInterval, "refresh_shutdowns", s.shutdowns.Refresh)
	})
	wg.Go(func() {
		s.run(ctx, s.conf.NotifyInterval, "notify_shutdown_updates", s.notifications.NotifyShutdownUpdates)
	})
	wg.Go(func() {
		s.run(ctx, s.conf.NotifyUpcomingInterval, "notify_upcoming_change", s.alerts.NotifyPowerSupplyChanges)
	})
	wg.Go(func() {
		s.run(ctx, s.conf.CleanupInterval, "cleanup", s.runCleanups)
	})

	wg.Wait()
}

func (s *Scheduler) runCleanups(ctx context.Context) error {
	s.log.Info("starting cleanups")
	err := s.notifications.Cleanup(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to cleanup notifications", "error", err)
	}
	err = s.alerts.Cleanup(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to cleanup alerts", "error", err)
	}
	return nil
}

func (s *Scheduler) run(ctx context.Context, interval time.Duration, process string, fn processFn) {
	log := s.log.With("process", process)
	defer func() {
		log.InfoContext(ctx, "Stopped scheduler")
	}()

	const heartbeatInterval = 5 * time.Minute
	now := time.Now()
	pastHeartbeat := now.Add(-heartbeatInterval)

	log.InfoContext(ctx, "Starting scheduler", "interval", interval)
	for {
		if now.After(pastHeartbeat) {
			log.InfoContext(ctx, "Process is still running")
			pastHeartbeat = now
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			err := withRecovery(ctx, fn, log)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					log.InfoContext(ctx, "Action execution interrupted", "error", err)
					continue
				}

				log.ErrorContext(ctx, "Failed to run process", "error", err)
			}
		}
	}
}

func withRecovery(ctx context.Context, fn processFn, log *slog.Logger) error {
	defer func() {
		if r := recover(); r != nil {
			log.ErrorContext(ctx, "Recovered from panic", "error", r)
		}
	}()
	return fn(ctx)
}
