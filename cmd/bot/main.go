package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

const refreshTableInterval = 5 * time.Minute
const notifyUpdatesInterval = 5 * time.Second

func main() {
	ctx := context.Background()

	store := dal.NewBoltDB("data/app.db")
	defer store.Close()

	log := mustLogger(os.Getenv("ENV") == "dev")

	bb := telegram.NewBotBuilder()

	sender := bb.Sender(purgeSubscriber(store)) // todo use my own lib
	shutdownsSvc := service.NewShutdowns(store, log)
	subscriptionsSvc := service.NewSubscription(store, log)
	notificationsSvc := service.NewNotifications(store, store, sender, log)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		refreshShutdowns(ctx, shutdownsSvc, log)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		notifyShutdownUpdates(ctx, notificationsSvc, log)
	}()

	slog.Info("Starting bot")
	bb.Build(subscriptionsSvc).Start()

	wg.Wait()
	slog.Info("Stopped bot")
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

func purgeSubscriber(store *dal.BoltDB) func(chatID int64) {
	return func(chatID int64) {
		if err := store.PurgeSubscriptions(chatID); err != nil {
			slog.Error("failed to purge subscription", "chatID", chatID, "error", err)
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
