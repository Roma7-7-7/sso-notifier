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
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

const refreshTableInterval = 5 * time.Minute
const notifyUpdatesInterval = time.Minute

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log := mustLogger(os.Getenv("ENV") == "dev")

	store, err := dal.NewBoltDB("data/app.db")
	if err != nil {
		log.Error("Failed to open database", err)
		os.Exit(1)
	}
	defer store.Close()

	sender := tc.NewClient(http.DefaultClient, os.Getenv("TOKEN"))
	shutdownsSvc := service.NewShutdowns(store, log)
	subscriptionsSvc := service.NewSubscription(store, log)
	notificationsSvc := service.NewNotifications(store, store, sender, log)

	bot, err := telegram.NewBot(os.Getenv("TOKEN"), subscriptionsSvc, log)
	if err != nil {
		log.Error("Failed to create telegram bot", err)
		os.Exit(1)
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		refreshShutdowns(ctx, shutdownsSvc, log.With("component", "schedule").With("action", "refresh"))
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifyShutdownUpdates(ctx, notificationsSvc, log.With("component", "schedule").With("action", "notify"))
	}()

	log.Info("Starting bot")
	err = bot.Start(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Error("Failed to start bot", err)
		}
	}

	wg.Wait()
	log.Info("Stopped bot")
}

func refreshShutdowns(ctx context.Context, svc *service.Shutdowns, log *slog.Logger) {
	defer func() {
		log.InfoContext(ctx, "Stopped refresh shutdowns schedule")
	}()

	log.InfoContext(ctx, "Starting refresh shutdowns schedule")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(refreshTableInterval):
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

func notifyShutdownUpdates(ctx context.Context, svc *service.Notifications, log *slog.Logger) {
	defer func() {
		log.InfoContext(ctx, "Stopped notify shutdown updates schedule")
	}()

	log.InfoContext(ctx, "Starting notify shutdown updates schedule")
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(notifyUpdatesInterval):
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
