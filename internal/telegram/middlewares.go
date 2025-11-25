package telegram

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/Roma7-7-7/telegram"
)

type PurgeOnForbiddenMiddleware struct {
	subscriptions Subscriptions

	log *slog.Logger
}

func NewPurgeOnForbiddenMiddleware(subscriptions Subscriptions, log *slog.Logger) *PurgeOnForbiddenMiddleware {
	return &PurgeOnForbiddenMiddleware{
		subscriptions: subscriptions,
		log:           log,
	}
}

func (m *PurgeOnForbiddenMiddleware) Handle(next telegram.Handler) telegram.Handler {
	return func(ctx telegram.Context) error {
		rootErr := next(ctx)
		if errors.Is(rootErr, telegram.ErrForbidden) {
			m.log.WarnContext(ctx, "Bot is blocked. Marking to purge")
			chatIDStr, ok := ctx.ChatID()
			if !ok {
				m.log.WarnContext(ctx, "ChatID is not present in telegram context")
				return rootErr
			}
			chatID, err := strconv.ParseInt(chatIDStr, 10, 64) //nolint:mnd
			if err != nil {
				m.log.WarnContext(ctx, "ChatID is not a number", "chatID", chatID, "error", err)
				return rootErr
			}
			err = m.subscriptions.Unsubscribe(chatID)
			if err != nil {
				m.log.ErrorContext(ctx, "Mark to purge failed", "chatID", chatID, "error", err)
				return rootErr
			}
		}
		return rootErr
	}
}
