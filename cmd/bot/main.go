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

	"github.com/Roma7-7-7/sso-notifier/internal/config"
	"go.etcd.io/bbolt"

	tc "github.com/Roma7-7-7/telegram"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
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
	conf, err := config.NewConfig(ctx)
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

	db, err := bbolt.Open(conf.DBPath, 0600, nil) //nolint:mnd // read_write
	if err != nil {
		log.ErrorContext(ctx, "Failed to open database", "error", err)
		return 1
	}

	c := clock.NewWithLocation(loc)

	store, err := dal.NewBoltDB(db, c)
	if err != nil {
		log.ErrorContext(ctx, "Failed to open database", "error", err)
		return 1
	}
	defer store.Close()

	log.InfoContext(ctx, "Running database migrations")
	if err := migrations.RunMigrations(db, log); err != nil {
		log.ErrorContext(ctx, "Failed to run database migrations", "error", err)
		return 1
	}

	sender := tc.NewClient(http.DefaultClient, conf.TelegramToken)
	provider := providers.NewChernivtsiProvider(conf.ScheduleURL)
	shutdownsSvc := service.NewShutdowns(store, provider, c, log)
	subscriptionsSvc := service.NewSubscription(store, c, log)
	notificationsSvc := service.NewNotifications(store, store, store, sender, c, conf.NotificationsStateTTL, log)
	alertsSvc := service.NewAlerts(store, store, store, sender, c, conf.AlertsTTL, log)
	handler := telegram.NewHandler(subscriptionsSvc, notificationsSvc, conf.GroupsCount, log)

	bot, err := telegram.NewBot(conf, handler, log)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create telegram bot", "error", err)
		return 1
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.NewScheduler(conf, shutdownsSvc, notificationsSvc, alertsSvc, log).Start(ctx)
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
