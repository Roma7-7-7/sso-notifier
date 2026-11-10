package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kelseyhightower/envconfig"

	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/internal/scheduleapi"
)

const shutdownTimeout = 5 * time.Second

type config struct {
	Dev         bool          `envconfig:"DEV" default:"false"`
	Addr        string        `envconfig:"SCHEDULE_API_ADDR" default:"127.0.0.1:47653"`
	Interval    time.Duration `envconfig:"REFRESH_SHUTDOWNS_INTERVAL" default:"5m"`
	ScheduleURL string        `envconfig:"SCHEDULE_URL" default:"https://oblenergo.cv.ua/shutdowns/"`
	ProxyToken  string        `envconfig:"PROXY_TOKEN" default:""`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if !run(ctx) {
		cancel()
		os.Exit(1) //nolint:gocritic // cancel is called explicitly above
	}
}

func run(ctx context.Context) bool {
	var conf config
	if err := envconfig.Process("", &conf); err != nil {
		slog.ErrorContext(ctx, "Failed to process config", "error", err) //nolint:sloglint // logger not initialized yet
		return false
	}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, nil)
	if conf.Dev {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	log := slog.New(handler)

	loc, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		log.ErrorContext(ctx, "Failed to load timezone", "error", err)
		return false
	}

	svc := scheduleapi.NewService(providers.NewChernivtsiProvider(conf.ScheduleURL, conf.ProxyToken), loc, conf.Interval, log)
	go svc.Run(ctx)

	srv := &http.Server{
		Addr:              conf.Addr,
		Handler:           svc.Handler(),
		ReadHeaderTimeout: 5 * time.Second, //nolint:mnd // short header timeout
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.InfoContext(ctx, "Schedule API listening", "addr", conf.Addr, "interval", conf.Interval)

	select {
	case err := <-errCh:
		log.ErrorContext(ctx, "Server failed", "error", err)
		return false
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.ErrorContext(ctx, "Shutdown failed", "error", err)
		return false
	}

	return true
}
