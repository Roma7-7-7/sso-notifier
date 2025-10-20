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

	"github.com/kelseyhightower/envconfig"

	tc "github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

type Config struct {
	Dev                      bool          `envconfig:"DEV" default:"false"`
	GroupsCount              int           `envconfig:"GROUPS_COUNT" default:"12"`
	DBPath                   string        `envconfig:"DB_PATH" default:"data/sso-notifier.db"`
	RefreshShutdownsInterval time.Duration `envconfig:"REFRESH_SHUTDOWNS_INTERVAL" default:"5m"`
	NotifyInterval           time.Duration `envconfig:"NOTIFY_INTERVAL" default:"5m"`
	TelegramToken            string        `envconfig:"TELEGRAM_TOKEN" required:"true"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conf := &Config{}
	err := envconfig.Process("", conf)
	if err != nil {
		slog.Error("Failed to process env vars", "error", err)
		os.Exit(1)
	}

	log := mustLogger(conf.Dev)

	store, err := dal.NewBoltDB(conf.DBPath)
	if err != nil {
		log.Error("Failed to open database", err)
		os.Exit(1)
	}
	defer store.Close()

	sender := tc.NewClient(http.DefaultClient, conf.TelegramToken)
	shutdownsSvc := service.NewShutdowns(store, log)
	subscriptionsSvc := service.NewSubscription(store, log)
	notificationsSvc := service.NewNotifications(store, store, sender, log)

	bot, err := telegram.NewBot(conf.TelegramToken, subscriptionsSvc, conf.GroupsCount, log)
	if err != nil {
		log.Error("Failed to create telegram bot", err)
		os.Exit(1)
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
