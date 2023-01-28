package main

import (
	"errors"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	tb "gopkg.in/telebot.v3"
)

const groupsCount = 18

type SSOBot struct {
	bot     *tb.Bot
	service Service
	markups *markups
}

func (b *SSOBot) Start() {
	b.bot.Handle("/start", b.StartHandler)
	for _, btn := range b.markups.backToMainBtns() {
		b.bot.Handle(&btn, b.StartHandler)
	}

	b.bot.Handle("/subscribe", b.ChooseGroupHandler)
	for _, btn := range b.markups.chooseGroupBtns() {
		b.bot.Handle(&btn, b.ChooseGroupHandler)
	}

	for k, btn := range b.markups.subscribeToGroupBtns() {
		b.bot.Handle(&btn, b.SetGroupHandler(k))
	}

	b.bot.Handle("/unsubscribe", b.UnsubscribeHandler)
	for _, btn := range b.markups.unsubscribeBtns() {
		b.bot.Handle(&btn, b.UnsubscribeHandler)
	}

	b.bot.Start()
}

func (b *SSOBot) StartHandler(c tb.Context) error {
	markup := b.markups.main.unsubscribed.ReplyMarkup
	subscribed, err := b.service.IsSubscribed(c.Sender().ID)
	if err != nil {
		zap.L().Error("failed to check if user is subscribed", zap.Error(err))
		return c.Send("Щось пішло не так. Будь ласка, спробуйте пізніше.")
	}
	if subscribed {
		markup = b.markups.main.subscribed.ReplyMarkup
	}
	return c.Send("Привіт! Бажаєте підписатись на оновлення графіку відключень?", markup)
}

func (b *SSOBot) ChooseGroupHandler(c tb.Context) error {
	return c.Send("Оберіть групу", b.markups.groups.ReplyMarkup)
}

func (b *SSOBot) SetGroupHandler(groupNumber string) func(c tb.Context) error {
	return func(c tb.Context) error {
		if _, err := b.service.SetGroup(c.Sender().ID, groupNumber); errors.Is(err, ErrSubscribersLimitReached) {
			zap.L().Warn("failed to subscribe", zap.Error(err), zap.String("groupNum", groupNumber))
			return c.Send("Кількість підписок досягла межі. Будь ласка, спробуйте пізніше.")
		} else if err != nil {
			zap.L().Error("failed to subscribe", zap.Error(err), zap.String("groupNum", groupNumber))
			return c.Send("Не вдалось підписатись. Будь ласка, спробуйте пізніше.")
		}

		return c.Send("Ви підписались на групу "+groupNumber, b.markups.main.subscribed.ReplyMarkup)
	}
}

func (b *SSOBot) UnsubscribeHandler(c tb.Context) error {
	if err := b.service.Unsubscribe(c.Sender().ID); err != nil {
		zap.L().Error("failed to unsubscribe", zap.Error(err))
		return c.Send("Не вдалось відписатись. Будь ласка, спробуйте пізніше.", b.markups.main.subscribed.ReplyMarkup)
	}
	return c.Send("Ви відписані", b.markups.main.unsubscribed.ReplyMarkup)
}

type tBotSender struct {
	bot *tb.Bot
}

func (s *tBotSender) Send(chatID int64, msg string) error {
	_, err := s.bot.Send(tb.ChatID(chatID), msg)
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
		Poller: &tb.LongPoller{Timeout: 5 * time.Second}, //nolint:gomnd
	})
	if err != nil {
		zap.L().Fatal("failed to create bot", zap.Error(err))
	}

	return bot
}

func NewBot(service Service, tbot *tb.Bot) *SSOBot {
	return &SSOBot{
		bot:     tbot,
		service: service,
		markups: newMarkups(),
	}
}

type subscribedMarkup struct {
	*tb.ReplyMarkup
	chooseOtherGroup tb.Btn
	unsubscribe      tb.Btn
}

type unsubscribedMarkup struct {
	*tb.ReplyMarkup
	subscribe tb.Btn
}

type mainMarkups struct {
	subscribed   subscribedMarkup
	unsubscribed unsubscribedMarkup
}

type groupsMarkup struct {
	*tb.ReplyMarkup
	subscribeGroupBtns map[string]tb.Btn
	backBtn            tb.Btn
}

type markups struct {
	main   mainMarkups
	groups groupsMarkup
}

func newMarkups() *markups {
	mainSubscribed := &tb.ReplyMarkup{}
	chooseOtherGroupBtn := mainSubscribed.Data("Обрати іншу групу", "choose_other_group")
	unsubscribeBtn := mainSubscribed.Data("Відписатись", "unsubscribe")
	mainSubscribed.Inline(
		mainSubscribed.Row(chooseOtherGroupBtn),
		mainSubscribed.Row(unsubscribeBtn),
	)

	mainUnsubscribed := &tb.ReplyMarkup{}
	subscribeBtn := mainUnsubscribed.Data("Підписатись на оновлення", "subscribe")
	mainUnsubscribed.Inline(mainUnsubscribed.Row(subscribeBtn))

	gm := &tb.ReplyMarkup{}
	const buttonsPerRow = 5
	groupBtns := make(map[string]tb.Btn, groupsCount)
	groupMarkupRows := make([]tb.Row, 0, groupsCount/buttonsPerRow+1)
	for i := 0; i < groupsCount; i++ {
		groupNum := strconv.Itoa(i + 1)
		groupBtns[groupNum] = gm.Data(groupNum, "subscribe_group_"+groupNum)

		rowIndex := i / buttonsPerRow
		if len(groupMarkupRows) <= rowIndex {
			groupMarkupRows = append(groupMarkupRows, tb.Row{})
		}
		groupMarkupRows[rowIndex] = append(groupMarkupRows[rowIndex], groupBtns[groupNum])
	}
	back := gm.Data("Назад", "back")
	groupMarkupRows = append(groupMarkupRows, tb.Row{back})
	gm.Inline(groupMarkupRows...)

	return &markups{
		main: mainMarkups{
			subscribed: subscribedMarkup{
				ReplyMarkup:      mainSubscribed,
				chooseOtherGroup: chooseOtherGroupBtn,
				unsubscribe:      unsubscribeBtn,
			},
			unsubscribed: unsubscribedMarkup{
				ReplyMarkup: mainUnsubscribed,
				subscribe:   subscribeBtn,
			},
		},
		groups: groupsMarkup{
			ReplyMarkup:        gm,
			subscribeGroupBtns: groupBtns,
			backBtn:            back,
		},
	}
}

func (m *markups) chooseGroupBtns() []tb.Btn {
	return []tb.Btn{
		m.main.subscribed.chooseOtherGroup,
		m.main.unsubscribed.subscribe,
	}
}

func (m *markups) unsubscribeBtns() []tb.Btn {
	return []tb.Btn{
		m.main.subscribed.unsubscribe,
	}
}

func (m *markups) subscribeToGroupBtns() map[string]tb.Btn {
	return m.groups.subscribeGroupBtns
}

func (m *markups) backToMainBtns() []tb.Btn {
	return []tb.Btn{m.groups.backBtn}
}
