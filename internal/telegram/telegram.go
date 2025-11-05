package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	GetSubscribedGroups(chatID int64) ([]string, error)
	ToggleGroupSubscription(chatID int64, number string) error
	Unsubscribe(chatID int64) error
	GetSettings(chatID int64) (map[dal.SettingKey]interface{}, error)
	ToggleSetting(chatID int64, key dal.SettingKey, defaultValue bool) error
	GetBoolSetting(chatID int64, key dal.SettingKey, defaultValue bool) (bool, error)
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
	b.bot.Handle("/subscribe", b.ManageGroupsHandler)
	b.bot.Handle("/unsubscribe", b.UnsubscribeHandler)
	b.bot.Handle("/settings", b.SettingsHandler)

	// Register catch-all callback handler FIRST
	b.bot.Handle(tb.OnCallback, b.handleCallbackRouter)

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

	b.log.Debug("start handler called",
		"chatID", chatID,
		"subscribed", subscribed)

	var message string
	var markup *tb.ReplyMarkup

	if subscribed {
		// Get subscribed groups
		groups, err := b.svc.GetSubscribedGroups(chatID)
		if err != nil {
			b.log.Error("failed to get subscribed groups",
				"error", err,
				"chatID", chatID)
			return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
		}

		// Build message with group list
		groupsList := formatGroupsList(groups)
		message = fmt.Sprintf("Привіт! Ви підписані на групи: %s", groupsList)
		markup = b.markups.main.subscribed.ReplyMarkup
	} else {
		message = "Привіт! Бажаєте підписатись на оновлення графіку відключень?"
		markup = b.markups.main.unsubscribed.ReplyMarkup
	}

	return b.sendOrDelete(c, message, markup)
}

func (b *Bot) ManageGroupsHandler(c tb.Context) error {
	chatID := c.Sender().ID
	b.log.Debug("manage groups handler called", "chatID", chatID)

	// Get current subscriptions
	subscribedGroups, err := b.svc.GetSubscribedGroups(chatID)
	if err != nil {
		b.log.Error("failed to get subscribed groups",
			"error", err,
			"chatID", chatID)
		return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
	}

	// Convert to map for quick lookup
	subscribedMap := make(map[string]bool)
	for _, groupNum := range subscribedGroups {
		subscribedMap[groupNum] = true
	}

	// Build dynamic markup with checkmarks
	markup := b.markups.buildDynamicGroupsMarkup(subscribedMap)

	return b.sendOrDelete(c, "Оберіть групи для підписки\n(натисніть щоб додати/видалити)", markup)
}

func (b *Bot) handleCallbackRouter(c tb.Context) error {
	callback := c.Callback()
	if callback == nil {
		b.log.Debug("callback router called with nil callback")
		return nil
	}

	chatID := c.Sender().ID
	b.log.Debug("callback received",
		"chatID", chatID,
		"data", callback.Data,
		"unique", callback.Unique,
		"messageID", callback.MessageID)

	// Respond to callback first to remove loading state
	if err := c.Respond(); err != nil {
		b.log.Warn("failed to respond to callback", "error", err, "chatID", chatID)
	}

	// Use Data field and trim the prefix if present
	data := callback.Data
	if len(data) > 0 && data[0] == '\f' {
		data = data[1:]
	}

	b.log.Debug("routing callback", "processedData", data)

	// Route based on callback data
	switch {
	case data == "subscribe", data == "manage_groups":
		b.log.Debug("routing to ManageGroupsHandler")
		return b.ManageGroupsHandler(c)

	case data == "unsubscribe":
		b.log.Debug("routing to UnsubscribeHandler")
		return b.UnsubscribeHandler(c)

	case data == "settings":
		b.log.Debug("routing to SettingsHandler")
		return b.SettingsHandler(c)

	case data == "toggle_notify_off":
		b.log.Debug("routing to ToggleSettingHandler for notify_off")
		return b.ToggleSettingHandler(dal.SettingNotifyOff)(c)

	case data == "toggle_notify_maybe":
		b.log.Debug("routing to ToggleSettingHandler for notify_maybe")
		return b.ToggleSettingHandler(dal.SettingNotifyMaybe)(c)

	case data == "toggle_notify_on":
		b.log.Debug("routing to ToggleSettingHandler for notify_on")
		return b.ToggleSettingHandler(dal.SettingNotifyOn)(c)

	case data == "back_from_settings":
		b.log.Debug("routing to StartHandler from settings")
		return b.StartHandler(c)

	case data == "back":
		b.log.Debug("routing to StartHandler")
		return b.StartHandler(c)

	case len(data) > 13 && data[:13] == "toggle_group_":
		groupNum := data[13:]
		b.log.Debug("routing to ToggleGroupHandler", "groupNum", groupNum)
		return b.ToggleGroupHandler(groupNum)(c)

	default:
		b.log.Debug("no handler matched for callback", "data", data)
		return nil
	}
}

func (b *Bot) ToggleGroupHandler(groupNumber string) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := b.svc.ToggleGroupSubscription(chatID, groupNumber); err != nil {
			b.log.Error("failed to toggle subscription",
				"error", err,
				"chatID", chatID,
				"groupNum", groupNumber)
			return b.sendOrDelete(c, "Не вдалось оновити підписку. Будь ласка, спробуйте пізніше.", nil)
		}

		// Get updated subscriptions
		subscribedGroups, err := b.svc.GetSubscribedGroups(chatID)
		if err != nil {
			b.log.Error("failed to get subscribed groups after toggle",
				"error", err,
				"chatID", chatID)
			return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
		}

		b.log.Info("user toggled group subscription",
			"chatID", chatID,
			"groupNum", groupNumber,
			"subscribedGroups", subscribedGroups)

		// Convert to map for quick lookup
		subscribedMap := make(map[string]bool)
		isSubscribed := false
		for _, gNum := range subscribedGroups {
			subscribedMap[gNum] = true
			if gNum == groupNumber {
				isSubscribed = true
			}
		}

		// Build updated markup
		markup := b.markups.buildDynamicGroupsMarkup(subscribedMap)

		// Show feedback message
		var message string
		if isSubscribed {
			message = fmt.Sprintf("✅ Підписано на групу %s\n\nОберіть групи для підписки\n(натисніть щоб додати/видалити)", groupNumber)
		} else {
			if len(subscribedGroups) == 0 {
				// User removed all groups - return to main menu
				return b.sendOrDelete(c, "Ви відписані від усіх груп", b.markups.main.unsubscribed.ReplyMarkup)
			}
			message = fmt.Sprintf("❌ Відписано від групи %s\n\nОберіть групи для підписки\n(натисніть щоб додати/видалити)", groupNumber)
		}

		return b.sendOrDelete(c, message, markup)
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

func (b *Bot) SettingsHandler(c tb.Context) error {
	chatID := c.Sender().ID
	b.log.Debug("settings handler called", "chatID", chatID)

	subscribed, err := b.svc.IsSubscribed(chatID)
	if err != nil {
		b.log.Error("failed to check subscription status",
			"error", err,
			"chatID", chatID)
		return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
	}

	if !subscribed {
		return b.sendOrDelete(c, "Налаштування доступні тільки для підписаних користувачів. Спочатку підпишіться на оновлення.", b.markups.main.unsubscribed.ReplyMarkup)
	}

	settings, err := b.svc.GetSettings(chatID)
	if err != nil {
		b.log.Error("failed to get settings",
			"error", err,
			"chatID", chatID)
		return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
	}

	markup := b.markups.buildSettingsMarkup(settings)

	message := "⚙️ Налаштування сповіщень\n\n" +
		"Попереджати за 10 хвилин до:\n\n" +
		"ℹ️ Сповіщення надсилаються з 6:00 до 23:00"

	return b.sendOrDelete(c, message, markup)
}

func (b *Bot) ToggleSettingHandler(settingKey dal.SettingKey) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := b.svc.ToggleSetting(chatID, settingKey, true); err != nil {
			b.log.Error("failed to toggle setting",
				"error", err,
				"chatID", chatID,
				"settingKey", settingKey)
			return b.sendOrDelete(c, "Не вдалось оновити налаштування. Будь ласка, спробуйте пізніше.", nil)
		}

		b.log.Info("user toggled setting",
			"chatID", chatID,
			"settingKey", settingKey)

		settings, err := b.svc.GetSettings(chatID)
		if err != nil {
			b.log.Error("failed to get settings after toggle",
				"error", err,
				"chatID", chatID)
			return b.sendOrDelete(c, "Щось пішло не так. Будь ласка, спробуйте пізніше.", nil)
		}

		markup := b.markups.buildSettingsMarkup(settings)

		message := "⚙️ Налаштування сповіщень\n\n" +
			"Попереджати за 10 хвилин до:\n\n" +
			"ℹ️ Сповіщення надсилаються з 6:00 до 23:00"

		return b.sendOrDelete(c, message, markup)
	}
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

type (
	// subscribedMarkup contains the markup for subscribed users
	subscribedMarkup struct {
		*tb.ReplyMarkup
		manageGroups tb.Btn
		settings     tb.Btn
		unsubscribe  tb.Btn
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

	// markups aggregates all keyboard markups used by the bot
	markups struct {
		main        mainMarkups
		groupsCount int
	}
)

func newMarkups(subscriptionGroupsCount int) *markups {
	// Create markup for subscribed users
	mainSubscribed := &tb.ReplyMarkup{}
	manageGroupsBtn := mainSubscribed.Data("Керувати групами", "manage_groups")
	subscribedSettingsBtn := mainSubscribed.Data("⚙️ Налаштування", "settings")
	unsubscribeBtn := mainSubscribed.Data("Відписатись від усіх", "unsubscribe")
	mainSubscribed.Inline(
		mainSubscribed.Row(manageGroupsBtn),
		mainSubscribed.Row(subscribedSettingsBtn),
		mainSubscribed.Row(unsubscribeBtn),
	)

	// Create markup for unsubscribed users
	mainUnsubscribed := &tb.ReplyMarkup{}
	subscribeBtn := mainUnsubscribed.Data("Підписатись на оновлення", "subscribe")
	mainUnsubscribed.Inline(mainUnsubscribed.Row(subscribeBtn))

	// Create group selection markup (static structure, will be rebuilt dynamically)

	return &markups{
		main: mainMarkups{
			subscribed: subscribedMarkup{
				ReplyMarkup:  mainSubscribed,
				manageGroups: manageGroupsBtn,
				settings:     subscribedSettingsBtn,
				unsubscribe:  unsubscribeBtn,
			},
			unsubscribed: unsubscribedMarkup{
				ReplyMarkup: mainUnsubscribed,
				subscribe:   subscribeBtn,
			},
		},
		groupsCount: subscriptionGroupsCount,
	}
}

// buildDynamicGroupsMarkup creates group selection keyboard with checkmarks for subscribed groups
func (m *markups) buildDynamicGroupsMarkup(subscribedGroups map[string]bool) *tb.ReplyMarkup {
	const (
		buttonsPerRow        = 4
		additionalRowsBuffer = 2
	)

	markup := &tb.ReplyMarkup{}
	rows := make([]tb.Row, 0, m.groupsCount/buttonsPerRow+additionalRowsBuffer)

	currentRow := tb.Row{}
	for i := range m.groupsCount {
		groupNum := strconv.Itoa(i + 1)

		// Add checkmark if subscribed
		btnText := groupNum
		if subscribedGroups[groupNum] {
			btnText = groupNum + " ✅"
		}

		btn := markup.Data(btnText, "toggle_group_"+groupNum)
		currentRow = append(currentRow, btn)

		if len(currentRow) == buttonsPerRow {
			rows = append(rows, currentRow)
			currentRow = tb.Row{}
		}
	}

	if len(currentRow) > 0 {
		rows = append(rows, currentRow)
	}

	backBtn := markup.Data("Назад", "back")
	rows = append(rows, markup.Row(backBtn))
	markup.Inline(rows...)

	return markup
}

// buildSettingsMarkup creates settings keyboard with checkmarks for enabled settings
func (m *markups) buildSettingsMarkup(settings map[dal.SettingKey]interface{}) *tb.ReplyMarkup {
	markup := &tb.ReplyMarkup{}

	notifyOff := dal.GetBoolSetting(settings, dal.SettingNotifyOff, false)
	notifyMaybe := dal.GetBoolSetting(settings, dal.SettingNotifyMaybe, false)
	notifyOn := dal.GetBoolSetting(settings, dal.SettingNotifyOn, false)

	offText := "Відключення"
	if notifyOff {
		offText = "✅ " + offText
	} else {
		offText = "❌ " + offText
	}

	maybeText := "Можливих відключень"
	if notifyMaybe {
		maybeText = "✅ " + maybeText
	} else {
		maybeText = "❌ " + maybeText
	}

	onText := "Відновлення"
	if notifyOn {
		onText = "✅ " + onText
	} else {
		onText = "❌ " + onText
	}

	offBtn := markup.Data(offText, "toggle_notify_off")
	maybeBtn := markup.Data(maybeText, "toggle_notify_maybe")
	onBtn := markup.Data(onText, "toggle_notify_on")
	backBtn := markup.Data("◀️ Назад", "back_from_settings")

	markup.Inline(
		markup.Row(offBtn),
		markup.Row(maybeBtn),
		markup.Row(onBtn),
		markup.Row(backBtn),
	)

	return markup
}

// formatGroupsList formats a list of group numbers as a comma-separated string
func formatGroupsList(groups []string) string {
	if len(groups) == 0 {
		return ""
	}

	// Sort groups numerically for consistent display
	sortedGroups := make([]int, 0, len(groups))
	for _, g := range groups {
		if num, err := strconv.Atoi(g); err == nil {
			sortedGroups = append(sortedGroups, num)
		}
	}

	// Simple bubble sort (fine for small arrays like 12 groups max)
	for i := 0; i < len(sortedGroups); i++ {
		for j := i + 1; j < len(sortedGroups); j++ {
			if sortedGroups[i] > sortedGroups[j] {
				sortedGroups[i], sortedGroups[j] = sortedGroups[j], sortedGroups[i]
			}
		}
	}

	// Convert back to strings using strings.Builder for performance
	if len(sortedGroups) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, num := range sortedGroups {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(strconv.Itoa(num))
	}

	return builder.String()
}
