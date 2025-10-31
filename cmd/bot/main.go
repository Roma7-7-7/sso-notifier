package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tc "github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	if status := run(ctx); status > 0 {
		cancel()
		os.Exit(status)
	}
	cancel()
}

func run(ctx context.Context) int {
	conf, err := telegram.NewConfig(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to create configuration", "error", err) //nolint:sloglint // not initialized yet
		return 1
	}

	log := mustLogger(conf.Dev)

	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.ErrorContext(ctx, "Failed to load timezone", "error", err)
		return 1
	}

	store, err := dal.NewBoltDB(conf.DBPath)
	if err != nil {
		log.ErrorContext(ctx, "Failed to open database", "error", err)
		return 1
	}
	defer store.Close()

	log.InfoContext(ctx, "Running database migrations")
	if err := migrations.RunMigrations(store.DB(), log); err != nil {
		log.ErrorContext(ctx, "Failed to run database migrations", "error", err)
		return 1
	}

	sender := tc.NewClient(http.DefaultClient, conf.TelegramToken)
	shutdownsSvc := service.NewShutdowns(store, loc, log)
	subscriptionsSvc := service.NewSubscription(store, log)
	notificationsSvc := service.NewNotifications(store, store, sender, loc, log)

	bot, err := telegram.NewBot(conf, subscriptionsSvc, log)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create telegram bot", "error", err)
		return 1
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		refreshShutdowns(ctx, shutdownsSvc, conf.RefreshShutdownsInterval, log.With("component", "schedule").With("action", "refresh"))
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifyShutdownUpdates(ctx, notificationsSvc, conf.NotifyInterval, log.With("component", "schedule").With("action", "notify"))
	}()

	log.InfoContext(ctx, "Starting bot")
	err = bot.Start(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.ErrorContext(ctx, "Failed to start bot", "error", err)
		}
	}

	wg.Wait()
	log.InfoContext(ctx, "Stopped bot")
	return 0
}

func refreshShutdowns(ctx context.Context, svc *service.Shutdowns, delay time.Duration, log *slog.Logger) {
	defer func() {
		log.InfoContext(ctx, "Stopped refresh shutdowns schedule")
	}()

	log.InfoContext(ctx, "Starting refresh shutdowns schedule")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			err := svc.Refresh(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, context.DeadlineExceeded) {
					log.WarnContext(ctx, "Error refreshing shutdowns", "error", err)
					continue
				}

				log.ErrorContext(ctx, "Error refreshing shutdowns", "error", err)
			}
		}
	}
}

func notifyShutdownUpdates(ctx context.Context, svc *service.Notifications, delay time.Duration, log *slog.Logger) {
	defer func() {
		log.InfoContext(ctx, "Stopped notify shutdown updates schedule")
	}()

	log.InfoContext(ctx, "Starting notify shutdown updates schedule")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			err := svc.NotifyShutdownUpdates(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				if errors.Is(err, context.DeadlineExceeded) {
					log.WarnContext(ctx, "Error notifying shutdown updates", "error", err)
					continue
				}

				log.ErrorContext(ctx, "Error notifying shutdowns schedule", "error", err)
			}
		}
	}
}

func mustLogger(dev bool) *slog.Logger {
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	if dev {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return slog.New(handler)
}
