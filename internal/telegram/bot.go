package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tb "gopkg.in/telebot.v3"

	"github.com/Roma7-7-7/sso-notifier/internal/config"
)

type Bot struct {
	bot *tb.Bot

	handler *Handler

	log *slog.Logger
}

func NewBot(config *config.Config, handler *Handler, log *slog.Logger) (*Bot, error) {
	bot, err := tb.NewBot(tb.Settings{
		Token:  config.TelegramToken,
		Poller: &tb.LongPoller{Timeout: 5 * time.Second}, //nolint:mnd // it's ok
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Bot{
		bot: bot,

		handler: handler,

		log: log.With("component", "bot"),
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	// Register command handlers
	b.bot.Handle("/start", b.handler.Start)
	b.bot.Handle("/subscribe", b.handler.ManageGroups)
	b.bot.Handle("/schedule", b.handler.GetSchedule)
	b.bot.Handle("/settings", b.handler.Settings)
	b.bot.Handle("/unsubscribe", b.handler.Unsubscribe)

	b.bot.Handle(tb.OnCallback, b.handler.Callback)

	go func() {
		<-ctx.Done()
		b.log.Info("Stopping bot")
		b.bot.Stop()
	}()

	b.bot.Start()

	return nil
}
