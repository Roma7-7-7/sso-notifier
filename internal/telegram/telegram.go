package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	tb "gopkg.in/telebot.v3"
)

const (
	groupsCount = 12

	// Button text constants
	btnTextChooseOtherGroup = "Обрати іншу групу"
	btnTextUnsubscribe      = "Відписатись"
	btnTextSubscribe        = "Підписатись на оновлення"
	btnTextBack             = "Назад"

	// Message text constants
	msgWelcome          = "Привіт! Бажаєте підписатись на оновлення графіку відключень?"
	msgChooseGroup      = "Оберіть групу"
	msgSubscribed       = "Ви підписались на групу "
	msgUnsubscribed     = "Ви відписані"
	msgErrorGeneric     = "Щось пішло не так. Будь ласка, спробуйте пізніше."
	msgErrorSubscribe   = "Не вдалось підписатись. Будь ласка, спробуйте пізніше."
	msgErrorUnsubscribe = "Не вдалось відписатись. Будь ласка, спробуйте пізніше."
)

type MessageSender interface {
	SendMessage(ctx context.Context, chatID, msg string) error
}

type SubscriptionService interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscriptions() ([]dal.Subscription, error)
	SubscribeToGroup(chatID int64, number string) (dal.Subscription, error)
	Unsubscribe(chatID int64) error
}

type Bot struct {
	svc SubscriptionService

	bot     *tb.Bot
	markups *markups

	log *slog.Logger
}

func NewBot(token string, svc SubscriptionService, log *slog.Logger) (*Bot, error) {
	bot, err := tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 5 * time.Second}, //nolint:mnd
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Bot{
		bot: bot,

		svc:     svc,
		markups: newMarkups(groupsCount),

		log: log.With("component", "bot"),
	}, nil
}

func (b *Bot) Start(ctx context.Context) error {
	// Register command handlers
	b.bot.Handle("/start", b.StartHandler)
	b.bot.Handle("/subscribe", b.ChooseGroupHandler)
	b.bot.Handle("/unsubscribe", b.UnsubscribeHandler)

	// Register button handlers
	b.registerButtonHandlers(b.markups.backToMainBtns(), b.StartHandler)
	b.registerButtonHandlers(b.markups.chooseGroupBtns(), b.ChooseGroupHandler)
	b.registerButtonHandlers(b.markups.unsubscribeBtns(), b.UnsubscribeHandler)

	// Register group subscription button handlers
	for groupNum, btn := range b.markups.subscribeToGroupBtns() {
		b.bot.Handle(&btn, b.SetGroupHandler(groupNum))
	}

	go func() {
		<-ctx.Done()
		_, err := b.bot.Close()
		if err != nil {
			b.log.Error("Failed to close telegram bot", "error", err)
		}
	}()

	b.bot.Start()

	return nil
}

func (b *Bot) StartHandler(c tb.Context) error {
	chatID := c.Sender().ID
	markup := b.markups.main.unsubscribed.ReplyMarkup

	subscribed, err := b.svc.IsSubscribed(chatID)
	if err != nil {
		b.log.Error("failed to check if user is subscribed",
			"error", err,
			"chatID", chatID)
		return c.Send(msgErrorGeneric)
	}

	if subscribed {
		markup = b.markups.main.subscribed.ReplyMarkup
	}

	b.log.Debug("start handler called",
		"chatID", chatID,
		"subscribed", subscribed)
	return c.Send(msgWelcome, markup)
}

func (b *Bot) ChooseGroupHandler(c tb.Context) error {
	b.log.Debug("choose group handler called", "chatID", c.Sender().ID)
	return c.Send(msgChooseGroup, b.markups.groups.ReplyMarkup)
}

func (b *Bot) SetGroupHandler(groupNumber string) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		_, err := b.svc.SubscribeToGroup(chatID, groupNumber)
		if err != nil {
			b.log.Error("failed to subscribe",
				"error", err,
				"chatID", chatID,
				"groupNum", groupNumber)
			return c.Send(msgErrorSubscribe)
		}

		b.log.Info("user subscribed to group",
			"chatID", chatID,
			"groupNum", groupNumber)
		return c.Send(msgSubscribed+groupNumber, b.markups.main.subscribed.ReplyMarkup)
	}
}

func (b *Bot) UnsubscribeHandler(c tb.Context) error {
	chatID := c.Sender().ID

	if err := b.svc.Unsubscribe(chatID); err != nil {
		b.log.Error("failed to unsubscribe",
			"error", err,
			"chatID", chatID)
		return c.Send(msgErrorUnsubscribe, b.markups.main.subscribed.ReplyMarkup)
	}

	b.log.Info("user unsubscribed", "chatID", chatID)
	return c.Send(msgUnsubscribed, b.markups.main.unsubscribed.ReplyMarkup)
}

// registerButtonHandlers registers the same handler for multiple buttons
func (b *Bot) registerButtonHandlers(buttons []tb.Btn, handler tb.HandlerFunc) {
	for i := range buttons {
		b.bot.Handle(&buttons[i], handler)
	}
}

type BlockedByUserHandler func(chatID int64)

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
	chooseOtherGroupBtn := mainSubscribed.Data(btnTextChooseOtherGroup, "choose_other_group")
	unsubscribeBtn := mainSubscribed.Data(btnTextUnsubscribe, "unsubscribe")
	mainSubscribed.Inline(
		mainSubscribed.Row(chooseOtherGroupBtn),
		mainSubscribed.Row(unsubscribeBtn),
	)

	mainUnsubscribed := &tb.ReplyMarkup{}
	subscribeBtn := mainUnsubscribed.Data(btnTextSubscribe, "subscribe")
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
	back := gm.Data(btnTextBack, "back")
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
