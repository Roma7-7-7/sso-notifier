package main

import (
	"errors"
	"os"
	"time"

	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

type SSOBot struct {
	bot    *tb.Bot
	store  SubscriberStore
	sender Sender

	subscribeMarkup *tb.ReplyMarkup
	subscribeBtn    *tb.Btn
}

func (b *SSOBot) Start() {
	b.bot.Handle("/start", b.StartHandler)
	b.bot.Handle("/subscribe", b.SubscribeHandler)

	b.bot.Handle(b.subscribeBtn, b.SubscribeHandler)

	b.bot.Start()
}

func (b *SSOBot) StartHandler(c tb.Context) error {
	return c.Send("Привіт! Бажаєте підписатись на оновлення графіку відключень?", b.subscribeMarkup)
}

func (b *SSOBot) SubscribeHandler(c tb.Context) error {
	var err error
	size, err := b.store.NumSubscribers()
	if err != nil {
		zap.L().Error("failed to get size of subscribers", zap.Error(err))
		return c.Send("Не вдалось підписатись. Будь ласка, спробуйте пізніше")
	}
	if size >= 100 {
		zap.L().Warn("too many subscribers", zap.Int("size", size))
		return c.Send("Надто багато підписників. Зверніться до адміністратора, або спробуйте пізніше")
	}
	if added, sErr := b.store.AddSubscriber(Subscriber{ChatID: c.Chat().ID}); sErr != nil {
		zap.L().Error("failed to add subscriber", zap.Error(sErr))
		return c.Send("Не вдалось підписатись. Будь ласка, спробуйте пізніше")
	} else if added {
		chat := c.Chat()
		zap.L().Debug("new subscriber", zap.Int64("chatID", chat.ID))
		return c.Send("Ви підписались!")
	}
	return c.Send("Ви вже підписані =)")
}

type tBotSender struct {
	bot *tb.Bot
}

func (s *tBotSender) Send(target Subscriber, msg string) error {
	_, err := s.bot.Send(tb.ChatID(target.ChatID), msg)
	if errors.Is(err, tb.ErrBlockedByUser) {
		return ErrBlockedByUser // Return custom error to not depend on bot framework in other places
	}
	return err
}

func mustTBot() *tb.Bot {
	token := os.Getenv("TOKEN")
	if token == "" {
		zap.L().Fatal("TOKEN environment variable is missing")
	}

	bot, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 5 * time.Second},
	})
	if err != nil {
		zap.L().Fatal("failed to create bot", zap.Error(err))
	}

	return bot
}

func NewBot(store SubscriberStore, sender Sender, tbot *tb.Bot) *SSOBot {
	subscribeMarkup := &tb.ReplyMarkup{}
	subscribeBtn := subscribeMarkup.Data("Підписатись на оновлення", "subscribe")
	subscribeMarkup.Inline(subscribeMarkup.Row(subscribeBtn))

	return &SSOBot{
		bot:    tbot,
		store:  store,
		sender: sender,

		subscribeMarkup: subscribeMarkup,
		subscribeBtn:    &subscribeBtn,
	}
}
