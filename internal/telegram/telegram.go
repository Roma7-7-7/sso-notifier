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

type MessageSender interface {
	SendMessage(ctx context.Context, chatID, msg string) error
}

type SubscriptionService interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscriptions() ([]dal.Subscription, error)
	SubscribeToGroup(chatID int64, number string) error
	Unsubscribe(chatID int64) error
}

type Bot struct {
	svc SubscriptionService

	bot     *tb.Bot
	markups *markups

	log *slog.Logger
}

func NewBot(config *Config, svc SubscriptionService, log *slog.Logger) (*Bot, error) {
	bot, err := tb.NewBot(tb.Settings{
		Token:  config.TelegramToken,
		Poller: &tb.LongPoller{Timeout: 5 * time.Second}, //nolint:mnd // it's ok
	})
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	return &Bot{
		bot: bot,

		svc:     svc,
		markups: newMarkups(config.GroupsCount),

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
		b.log.Info("Stopping bot")
		b.bot.Stop()
	}()

	b.bot.Start()

	return nil
}

func (b *Bot) StartHandler(c tb.Context) error {
	chatID := c.Sender().ID

	subscribed, err := b.svc.IsSubscribed(chatID)
	if err != nil {
		b.log.Error("failed to check if user is subscribed",
			"error", err,
			"chatID", chatID)
		return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
	}

	markup := b.markups.main.unsubscribed.ReplyMarkup
	if subscribed {
		markup = b.markups.main.subscribed.ReplyMarkup
	}

	b.log.Debug("start handler called",
		"chatID", chatID,
		"subscribed", subscribed)

	return b.sendOrDelete(c, "Привіт! Бажаєте підписатись на оновлення графіку відключень?", markup)
}

func (b *Bot) ChooseGroupHandler(c tb.Context) error {
	b.log.Debug("choose group handler called", "chatID", c.Sender().ID)
	return b.sendOrDelete(c, "Оберіть групу", b.markups.groups.ReplyMarkup)
}

func (b *Bot) SetGroupHandler(groupNumber string) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := b.svc.SubscribeToGroup(chatID, groupNumber); err != nil {
			b.log.Error("failed to subscribe",
				"error", err,
				"chatID", chatID,
				"groupNum", groupNumber)
			return b.sendOrDelete(c, "Не вдалось підписатись. Будь ласка, спробуйте пізніше.", nil)
		}

		b.log.Info("user subscribed to group",
			"chatID", chatID,
			"groupNum", groupNumber)

		message := fmt.Sprintf("Ви підписались на групу %s", groupNumber)
		return b.sendOrDelete(c, message, b.markups.main.subscribed.ReplyMarkup)
	}
}

func (b *Bot) UnsubscribeHandler(c tb.Context) error {
	chatID := c.Sender().ID

	if err := b.svc.Unsubscribe(chatID); err != nil {
		b.log.Error("failed to unsubscribe",
			"error", err,
			"chatID", chatID)
		return b.sendOrDelete(c, "Не вдалось відписатись. Будь ласка, спробуйте пізніше.", b.markups.main.subscribed.ReplyMarkup)
	}

	b.log.Info("user unsubscribed", "chatID", chatID)
	return b.sendOrDelete(c, "Ви відписані", b.markups.main.unsubscribed.ReplyMarkup)
}

// sendOrDelete deletes the previous message for callbacks and sends a new one
func (b *Bot) sendOrDelete(c tb.Context, text string, markup *tb.ReplyMarkup) error {
	// Check if this is a callback query (button press)
	if c.Callback() != nil {
		// Delete the old message to keep chat clean
		if err := c.Delete(); err != nil {
			b.log.Warn("failed to delete message",
				"error", err,
				"chatID", c.Sender().ID,
				"messageID", c.Message().ID)
		}
	}

	// Send new message (for both callbacks and commands)
	return c.Send(text, markup)
}

// registerButtonHandlers registers the same handler for multiple buttons
func (b *Bot) registerButtonHandlers(buttons []tb.Btn, handler tb.HandlerFunc) {
	for i := range buttons {
		b.bot.Handle(&buttons[i], handler)
	}
}

type (
	// subscribedMarkup contains the markup for subscribed users
	subscribedMarkup struct {
		*tb.ReplyMarkup
		chooseOtherGroup tb.Btn
		unsubscribe      tb.Btn
	}

	// unsubscribedMarkup contains the markup for unsubscribed users
	unsubscribedMarkup struct {
		*tb.ReplyMarkup
		subscribe tb.Btn
	}

	// mainMarkups holds both subscribed and unsubscribed markups
	mainMarkups struct {
		subscribed   subscribedMarkup
		unsubscribed unsubscribedMarkup
	}

	// groupsMarkup contains the group selection markup
	groupsMarkup struct {
		*tb.ReplyMarkup
		subscribeGroupBtns map[string]tb.Btn
		backBtn            tb.Btn
	}

	// markups aggregates all keyboard markups used by the bot
	markups struct {
		main   mainMarkups
		groups groupsMarkup
	}
)

func newMarkups(subscriptionGroupsCount int) *markups {
	// Create markup for subscribed users
	mainSubscribed := &tb.ReplyMarkup{}
	chooseOtherGroupBtn := mainSubscribed.Data("Обрати іншу групу", "choose_other_group")
	unsubscribeBtn := mainSubscribed.Data("Відписатись", "unsubscribe")
	mainSubscribed.Inline(
		mainSubscribed.Row(chooseOtherGroupBtn),
		mainSubscribed.Row(unsubscribeBtn),
	)

	// Create markup for unsubscribed users
	mainUnsubscribed := &tb.ReplyMarkup{}
	subscribeBtn := mainUnsubscribed.Data("Підписатись на оновлення", "subscribe")
	mainUnsubscribed.Inline(mainUnsubscribed.Row(subscribeBtn))

	// Create group selection markup
	groupsMarkup := buildGroupsMarkup(subscriptionGroupsCount)

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
		groups: groupsMarkup,
	}
}

// buildGroupsMarkup creates the group selection keyboard with configurable number of groups
func buildGroupsMarkup(groupsCount int) groupsMarkup {
	const (
		buttonsPerRow        = 5
		additionalRowsBuffer = 2 // Buffer for potential partial row and back button
	)

	markup := &tb.ReplyMarkup{}
	groupBtns := make(map[string]tb.Btn, groupsCount)
	rows := make([]tb.Row, 0, groupsCount/buttonsPerRow+additionalRowsBuffer)

	// Build group selection buttons
	currentRow := tb.Row{}
	for i := range groupsCount {
		groupNum := strconv.Itoa(i + 1)
		btn := markup.Data(groupNum, "subscribe_group_"+groupNum)
		groupBtns[groupNum] = btn
		currentRow = append(currentRow, btn)

		// Create new row when we reach buttonsPerRow
		if len(currentRow) == buttonsPerRow {
			rows = append(rows, currentRow)
			currentRow = tb.Row{}
		}
	}

	// Add remaining buttons if any
	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	// Add back button
	backBtn := markup.Data("Назад", "back")
	rows = append(rows, markup.Row(backBtn))

	markup.Inline(rows...)

	return groupsMarkup{
		ReplyMarkup:        markup,
		subscribeGroupBtns: groupBtns,
		backBtn:            backBtn,
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
