package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	tb "gopkg.in/telebot.v3"
)

const GroupsCount = 12

type MessageSender interface {
	SendMessage(ctx context.Context, chatID, msg string) error
}

type MessageSenderSetter interface {
	Set(MessageSender)
}

type SubscriptionService interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscriptions() ([]dal.Subscription, error)
	SubscribeToGroup(chatID int64, number string) (dal.Subscription, error)
	Unsubscribe(chatID int64) error
}

type SSOBot struct {
	bot     *tb.Bot
	markups *markups

	subscriptionService SubscriptionService
}

func (b *SSOBot) Start() {
	b.bot.Handle("/start", b.StartHandler)
	for _, btn := range b.markups.backToMainBtns() {
		btn := btn
		b.bot.Handle(&btn, b.StartHandler)
	}

	b.bot.Handle("/subscribe", b.ChooseGroupHandler)
	for _, btn := range b.markups.chooseGroupBtns() {
		btn := btn
		b.bot.Handle(&btn, b.ChooseGroupHandler)
	}

	for k, btn := range b.markups.subscribeToGroupBtns() {
		btn := btn
		b.bot.Handle(&btn, b.SetGroupHandler(k))
	}

	b.bot.Handle("/unsubscribe", b.UnsubscribeHandler)
	for _, btn := range b.markups.unsubscribeBtns() {
		btn := btn
		b.bot.Handle(&btn, b.UnsubscribeHandler)
	}

	b.bot.Start()
}

func (b *SSOBot) StartHandler(c tb.Context) error {
	markup := b.markups.main.unsubscribed.ReplyMarkup
	subscribed, err := b.subscriptionService.IsSubscribed(c.Sender().ID)
	if err != nil {
		slog.Error("failed to check if user is subscribed", "error", err)
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
		_, err := b.subscriptionService.SubscribeToGroup(c.Sender().ID, groupNumber)
		if err != nil {
			slog.Error("failed to subscribe", "error", err, "groupNum", groupNumber)
			return c.Send("Не вдалось підписатись. Будь ласка, спробуйте пізніше.")
		}

		return c.Send("Ви підписались на групу "+groupNumber, b.markups.main.subscribed.ReplyMarkup)
	}
}

func (b *SSOBot) UnsubscribeHandler(c tb.Context) error {
	if err := b.subscriptionService.Unsubscribe(c.Sender().ID); err != nil {
		slog.Error("failed to unsubscribe", "error", err)
		return c.Send("Не вдалось відписатись. Будь ласка, спробуйте пізніше.", b.markups.main.subscribed.ReplyMarkup)
	}
	return c.Send("Ви відписані", b.markups.main.unsubscribed.ReplyMarkup)
}

type SSOBotBuilder struct {
	bot *tb.Bot
}

func (bb *SSOBotBuilder) Sender(handler BlockedByUserHandler) MessageSender {
	return &messageSender{
		bot:            bb.bot,
		blockedHandler: handler,
	}
}

func (bb *SSOBotBuilder) Build(subscriptionService SubscriptionService) *SSOBot {
	return &SSOBot{
		bot:     bb.bot,
		markups: newMarkups(GroupsCount),

		subscriptionService: subscriptionService,
	}
}

type BlockedByUserHandler func(chatID int64)

func NewBotBuilder() *SSOBotBuilder {
	return &SSOBotBuilder{
		bot: mustTBot(),
	}
}

func mustTBot() *tb.Bot {
	token := os.Getenv("TOKEN")
	if token == "" {
		slog.Error("TOKEN environment variable is missing")
		panic("TOKEN environment variable is missing")
	}

	bot, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 5 * time.Second}, //nolint:gomnd
	})
	if err != nil {
		slog.Error("failed to create bot", "error", err)
		panic(fmt.Errorf("create bot: %w", err))
	}

	return bot
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

func newMarkups(subscriptionGroupsCount int) *markups {
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
	groupBtns := make(map[string]tb.Btn, subscriptionGroupsCount)
	groupMarkupRows := make([]tb.Row, 0, subscriptionGroupsCount/buttonsPerRow+1)
	for i := 0; i < subscriptionGroupsCount; i++ {
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

type messageSender struct {
	bot            *tb.Bot
	blockedHandler BlockedByUserHandler
}

func (s *messageSender) SendMessage(_ context.Context, chatIDStr string, msg string) error {
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %s", chatIDStr)
	}

	_, err = s.bot.Send(tb.ChatID(chatID), msg)
	if errors.Is(err, tb.ErrBlockedByUser) {
		slog.Debug("bot is banned, removing subscriber and all related data", "chatID", chatID)
		s.blockedHandler(chatID)
		return nil
	}
	return err
}
