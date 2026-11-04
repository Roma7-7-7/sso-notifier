package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tb "gopkg.in/telebot.v3"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
)

//go:generate mockgen -package mocks -destination mocks/telebot.go -mock_names Context=MockTelebotContext gopkg.in/telebot.v3/ Context

//go:generate mockgen -package mocks -destination mocks/subscriptions.go . Subscriptions

//go:generate mockgen -package mocks -destination mocks/notifications.go . Notifications

//go:generate mockgen -package mocks -destination mocks/emergency.go . EmergencyStore

const genericErrorMsg = "Щось пішло не так. Будь ласка, спробуйте пізніше."

type Subscriptions interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscribedGroups(chatID int64) ([]string, error)
	ToggleGroupSubscription(chatID int64, number string) error
	Unsubscribe(chatID int64) error
	GetSettings(chatID int64) (map[dal.SettingKey]interface{}, error)
	ToggleSetting(chatID int64, key dal.SettingKey, defaultValue bool) error
	SetSetting(chatID int64, key dal.SettingKey, value interface{}) error
}

type Notifications interface {
	NotifyPowerSupplySchedule(ctx context.Context, chatID int64) error
}

type EmergencyStore interface {
	GetEmergencyState() (dal.EmergencyState, error)
}

type Handler struct {
	subscriptions Subscriptions
	notifications Notifications
	emergency     EmergencyStore

	markups *markups

	log *slog.Logger
}

func NewHandler(subscriptions Subscriptions, notifications Notifications, emergency EmergencyStore, groupsCount int, log *slog.Logger) *Handler {
	return &Handler{
		subscriptions: subscriptions,
		notifications: notifications,
		emergency:     emergency,
		markups:       newMarkups(groupsCount),
		log:           log,
	}
}

func (h *Handler) Start(c tb.Context) error {
	chatID := c.Sender().ID

	subscribed, err := h.subscriptions.IsSubscribed(chatID)
	if err != nil {
		h.log.Error("failed to check if user is subscribed",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	h.log.Debug("start handler called",
		"chatID", chatID,
		"subscribed", subscribed)

	var message string
	var markup *tb.ReplyMarkup

	if subscribed {
		// Get subscribed groups
		groups, err := h.subscriptions.GetSubscribedGroups(chatID)
		if err != nil {
			h.log.Error("failed to get subscribed groups",
				"error", err,
				"chatID", chatID)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		// Build message with group list
		groupsList := formatGroupsList(groups)
		message = fmt.Sprintf("Привіт! Ви підписані на групи: %s", groupsList)
		markup = h.markups.main.subscribed.ReplyMarkup
	} else {
		message = "Привіт! Бажаєте підписатись на оновлення графіку відключень?"
		markup = h.markups.main.unsubscribed.ReplyMarkup
	}

	return h.sendOrDelete(c, message, markup)
}

func (h *Handler) ManageGroups(c tb.Context) error {
	chatID := c.Sender().ID
	h.log.Debug("manage groups handler called", "chatID", chatID)

	// Get current subscriptions
	subscribedGroups, err := h.subscriptions.GetSubscribedGroups(chatID)
	if err != nil {
		h.log.Error("failed to get subscribed groups",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	// Convert to map for quick lookup
	subscribedMap := make(map[string]bool)
	for _, groupNum := range subscribedGroups {
		subscribedMap[groupNum] = true
	}

	// Build dynamic markup with checkmarks
	markup := h.markups.buildDynamicGroupsMarkup(subscribedMap)

	return h.sendOrDelete(c, "Оберіть групи для підписки\n(натисніть щоб додати/видалити)", markup)
}

func (h *Handler) ToggleGroupHandler(groupNumber string) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := h.subscriptions.ToggleGroupSubscription(chatID, groupNumber); err != nil {
			h.log.Error("failed to toggle subscription",
				"error", err,
				"chatID", chatID,
				"groupNum", groupNumber)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		// Get updated subscriptions
		subscribedGroups, err := h.subscriptions.GetSubscribedGroups(chatID)
		if err != nil {
			h.log.Error("failed to get subscribed groups after toggle",
				"error", err,
				"chatID", chatID)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		h.log.Info("user toggled group subscription",
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
		markup := h.markups.buildDynamicGroupsMarkup(subscribedMap)

		// Show feedback message
		var message string
		if isSubscribed {
			message = fmt.Sprintf("✅ Підписано на групу %s\n\nОберіть групи для підписки\n(натисніть щоб додати/видалити)", groupNumber)
		} else {
			if len(subscribedGroups) == 0 {
				// User removed all groups - return to main menu
				return h.sendOrDelete(c, "Ви відписані від усіх груп", h.markups.main.unsubscribed.ReplyMarkup)
			}
			message = fmt.Sprintf("❌ Відписано від групи %s\n\nОберіть групи для підписки\n(натисніть щоб додати/видалити)", groupNumber)
		}

		return h.sendOrDelete(c, message, markup)
	}
}

func (h *Handler) Settings(c tb.Context) error {
	chatID := c.Sender().ID
	h.log.Debug("settings handler called", "chatID", chatID)

	subscribed, err := h.subscriptions.IsSubscribed(chatID)
	if err != nil {
		h.log.Error("failed to check subscription status",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	if !subscribed {
		return h.sendOrDelete(c, "Налаштування доступні тільки для підписаних користувачів. Спочатку підпишіться на оновлення.", h.markups.main.unsubscribed.ReplyMarkup)
	}

	markup := h.markups.buildSettingsMainMarkup()

	message := "⚙️ Налаштування\n\n" +
		"Оберіть розділ налаштувань:"

	return h.sendOrDelete(c, message, markup)
}

// scheduleTimeout covers one photo per group per date (up to 12 groups x 2 dates), each with a send interval and upload.
const scheduleTimeout = 2 * time.Minute

func (h *Handler) GetSchedule(c tb.Context) error {
	chatID := c.Sender().ID
	h.log.Debug("schedule handler called", "chatID", chatID)

	emergencyState, _ := h.emergency.GetEmergencyState()
	if emergencyState.Active {
		return h.sendOrDelete(c, "⚠️⚠️⚠️\nЗапроваджено екстренні відключення по Чернівецькій області. \nГрафіки погодинних відключень тимчасово не діють.\n⚠️⚠️⚠️", nil)
	}

	ctx, cancel := context.WithTimeout(context.Background(), scheduleTimeout)
	defer cancel()
	err := h.notifications.NotifyPowerSupplySchedule(ctx, chatID)
	if err != nil {
		if errors.Is(err, service.ErrSubscriptionNotFound) {
			return h.sendOrDelete(c, "Графік електропостачання доступний тільки для підписаних користувачів. Спочатку підпишіться на оновлення.", nil)
		}
		h.log.Error("failed to notify power supply schedule", "chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}
	return nil
}

func (h *Handler) SettingsAlerts(c tb.Context) error {
	chatID := c.Sender().ID
	h.log.Debug("settings alerts handler called", "chatID", chatID)

	subscribed, err := h.subscriptions.IsSubscribed(chatID)
	if err != nil {
		h.log.Error("failed to check subscription status",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	if !subscribed {
		return h.sendOrDelete(c, "Налаштування доступні тільки для підписаних користувачів. Спочатку підпишіться на оновлення.", h.markups.main.unsubscribed.ReplyMarkup)
	}

	settings, err := h.subscriptions.GetSettings(chatID)
	if err != nil {
		h.log.Error("failed to get settings",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	markup := h.markups.buildSettingsAlertsMarkup(settings)

	message := "🔔 Попереджати за 10 хвилин до:\n\n" +
		"ℹ️ Сповіщення надсилаються з 6:00 до 23:00"

	return h.sendOrDelete(c, message, markup)
}

func (h *Handler) SettingsNotificationsFormat(c tb.Context) error {
	chatID := c.Sender().ID
	h.log.Debug("settings notifications format handler called", "chatID", chatID)

	subscribed, err := h.subscriptions.IsSubscribed(chatID)
	if err != nil {
		h.log.Error("failed to check subscription status",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	if !subscribed {
		return h.sendOrDelete(c, "Налаштування доступні тільки для підписаних користувачів. Спочатку підпішіться на оновлення.", h.markups.main.unsubscribed.ReplyMarkup)
	}

	settings, err := h.subscriptions.GetSettings(chatID)
	if err != nil {
		h.log.Error("failed to get settings",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	markup := h.markups.buildSettingsNotificationsFormatMarkup(settings)

	message := "📋 Формат повідомлень про зміни графіку\n\n" +
		"Оберіть як відображати зміни графіку:\n\n" +
		"📌 Лінійний:\n" +
		"🟢 12:00 | 🔴 14:30 | 🟢 18:00\n\n" +
		"📌 Лінійний (деталізований):\n" +
		"🟢 12:00 - 14:30 | 🔴 14:30 - 18:00 | 🟢 18:00 - 21:00\n\n" +
		"📌 Згрупований:\n" +
		"  🟢 Заживлено: 12:00 - 14:30; 18:00 - 21:00;\n" +
		"  🔴 Відключено: 14:30 - 18:00;\n\n" +
		"📌 Картинка:\n" +
		"Погодинна сітка доби (текстом, якщо картинку не вдалося створити)"

	return h.sendOrDelete(c, message, markup)
}

func (h *Handler) SetFormatHandler(format string) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := h.subscriptions.SetSetting(chatID, dal.SettingShutdownsMessageFormat, format); err != nil {
			h.log.Error("failed to set format",
				"error", err,
				"chatID", chatID,
				"format", format)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		h.log.Info("user set format",
			"chatID", chatID,
			"format", format)

		settings, err := h.subscriptions.GetSettings(chatID)
		if err != nil {
			h.log.Error("failed to get settings after setting format",
				"error", err,
				"chatID", chatID)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		markup := h.markups.buildSettingsNotificationsFormatMarkup(settings)

		message := "✅ Формат повідомлень оновлено\n\n" +
			"📋 Формат повідомлень про зміни графіку\n\n" +
			"Оберіть як відображати зміни графіку:\n\n" +
			"📌 Лінійний:\n" +
			"🟢 12:00 | 🔴 14:30 | 🟢 18:00\n\n" +
			"📌 Лінійний, розширений:\n" +
			"🟢 12:00 - 14:30 | 🔴 14:30 - 18:00 | 🟢 18:00 - 21:00\n\n" +
			"📌 Згрупований:\n" +
			"  🟢 Заживлено: 12:00 - 14:30; 18:00 - 21:00;\n" +
			"  🔴 Відключено: 14:30 - 18:00;\n\n" +
			"📌 Картинка:\n" +
			"Погодинна сітка доби (текстом, якщо картинку не вдалося створити)"

		return h.sendOrDelete(c, message, markup)
	}
}

func (h *Handler) ToggleSettingHandler(settingKey dal.SettingKey) func(c tb.Context) error {
	return func(c tb.Context) error {
		chatID := c.Sender().ID

		if err := h.subscriptions.ToggleSetting(chatID, settingKey, true); err != nil {
			h.log.Error("failed to toggle setting",
				"error", err,
				"chatID", chatID,
				"settingKey", settingKey)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		h.log.Info("user toggled setting",
			"chatID", chatID,
			"settingKey", settingKey)

		settings, err := h.subscriptions.GetSettings(chatID)
		if err != nil {
			h.log.Error("failed to get settings after toggle",
				"error", err,
				"chatID", chatID)
			return h.sendOrDelete(c, genericErrorMsg, nil)
		}

		markup := h.markups.buildSettingsAlertsMarkup(settings)

		message := "🔔 Попереджати за 10 хвилин до:\n\n" +
			"ℹ️ Сповіщення надсилаються з 6:00 до 23:00"

		return h.sendOrDelete(c, message, markup)
	}
}

func (h *Handler) Callback(c tb.Context) error {
	callback := c.Callback()
	if callback == nil {
		h.log.Debug("callback router called with nil callback")
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}

	chatID := c.Sender().ID
	h.log.Debug("callback received",
		"chatID", chatID,
		"data", callback.Data,
		"unique", callback.Unique,
		"messageID", callback.MessageID)

	// Respond to callback first to remove loading state
	if err := c.Respond(); err != nil {
		h.log.Warn("failed to respond to callback", "error", err, "chatID", chatID)
	}

	// Use Data field and trim the prefix if present
	data := callback.Data
	if len(data) > 0 && data[0] == '\f' {
		data = data[1:]
	}

	h.log.Debug("routing callback", "processedData", data)

	// Route based on callback data
	switch {
	case data == "subscribe", data == "manage_groups":
		h.log.Debug("routing to ManageGroups")
		return h.ManageGroups(c)

	case data == "unsubscribe":
		h.log.Debug("routing to Unsubscribe")
		return h.Unsubscribe(c)

	case data == "settings":
		h.log.Debug("routing to Settings")
		return h.Settings(c)

	case data == "settings_alerts":
		h.log.Debug("routing to SettingsAlerts")
		return h.SettingsAlerts(c)

	case data == "settings_notifications_format":
		h.log.Debug("routing to SettingsNotificationsFormat")
		return h.SettingsNotificationsFormat(c)

	case data == "set_format_linear":
		h.log.Debug("routing to SetFormatHandler for linear")
		return h.SetFormatHandler(dal.ShutdownsMessageFormatLinear)(c)

	case data == "set_format_linear_with_range":
		h.log.Debug("routing to SetFormatHandler for linear_with_range")
		return h.SetFormatHandler(dal.ShutdownsMessageFormatLinearWithRange)(c)

	case data == "set_format_grouped":
		h.log.Debug("routing to SetFormatHandler for grouped")
		return h.SetFormatHandler(dal.ShutdownsMessageFormatGrouped)(c)

	case data == "set_format_image":
		h.log.Debug("routing to SetFormatHandler for image")
		return h.SetFormatHandler(dal.ShutdownsMessageFormatImage)(c)

	case data == "toggle_notify_off":
		h.log.Debug("routing to ToggleSettingHandler for notify_off")
		return h.ToggleSettingHandler(dal.SettingNotifyOff)(c)

	case data == "toggle_notify_maybe":
		h.log.Debug("routing to ToggleSettingHandler for notify_maybe")
		return h.ToggleSettingHandler(dal.SettingNotifyMaybe)(c)

	case data == "toggle_notify_on":
		h.log.Debug("routing to ToggleSettingHandler for notify_on")
		return h.ToggleSettingHandler(dal.SettingNotifyOn)(c)

	case data == "back_from_alerts":
		h.log.Debug("routing to Settings from alerts")
		return h.Settings(c)

	case data == "back_from_format":
		h.log.Debug("routing to Settings from format")
		return h.Settings(c)

	case data == "back_from_settings":
		h.log.Debug("routing to StartHandler from settings")
		return h.Start(c)

	case data == "back":
		h.log.Debug("routing to StartHandler")
		return h.Start(c)

	case len(data) > 13 && data[:13] == "toggle_group_":
		groupNum := data[13:]
		h.log.Debug("routing to ToggleGroupHandler", "groupNum", groupNum)
		return h.ToggleGroupHandler(groupNum)(c)

	default:
		h.log.Info("no handler matched for callback", "data", data)
		return h.sendOrDelete(c, genericErrorMsg, nil)
	}
}

func (h *Handler) Unsubscribe(c tb.Context) error {
	chatID := c.Sender().ID

	if err := h.subscriptions.Unsubscribe(chatID); err != nil {
		h.log.Error("failed to unsubscribe",
			"error", err,
			"chatID", chatID)
		return h.sendOrDelete(c, genericErrorMsg, h.markups.main.subscribed.ReplyMarkup)
	}

	h.log.Info("user unsubscribed", "chatID", chatID)
	return h.sendOrDelete(c, "Ви відписані", h.markups.main.unsubscribed.ReplyMarkup)
}

// sendOrDelete deletes the previous message for callbacks and sends a new one
func (h *Handler) sendOrDelete(c tb.Context, text string, markup *tb.ReplyMarkup) error {
	// Check if this is a callback query (button press)
	if c.Callback() != nil {
		// Delete the old message to keep chat clean
		if err := c.Delete(); err != nil {
			h.log.Warn("failed to delete message",
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

// buildSettingsMainMarkup creates main settings menu keyboard
func (m *markups) buildSettingsMainMarkup() *tb.ReplyMarkup {
	markup := &tb.ReplyMarkup{}

	alertsBtn := markup.Data("🔔 Попередження", "settings_alerts")
	formatBtn := markup.Data("📋 Формат повідомлень", "settings_notifications_format")
	backBtn := markup.Data("◀️ Назад", "back_from_settings")

	markup.Inline(
		markup.Row(alertsBtn),
		markup.Row(formatBtn),
		markup.Row(backBtn),
	)

	return markup
}

// buildSettingsAlertsMarkup creates alerts settings keyboard with checkmarks for enabled settings
func (m *markups) buildSettingsAlertsMarkup(settings map[dal.SettingKey]interface{}) *tb.ReplyMarkup {
	markup := &tb.ReplyMarkup{}

	notifyOff := dal.GetBoolSetting(settings, dal.SettingNotifyOff, false)
	notifyMaybe := dal.GetBoolSetting(settings, dal.SettingNotifyMaybe, false)
	notifyOn := dal.GetBoolSetting(settings, dal.SettingNotifyOn, false)

	offText := "Відключень"
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

	onText := "Відновлень"
	if notifyOn {
		onText = "✅ " + onText
	} else {
		onText = "❌ " + onText
	}

	offBtn := markup.Data(offText, "toggle_notify_off")
	maybeBtn := markup.Data(maybeText, "toggle_notify_maybe")
	onBtn := markup.Data(onText, "toggle_notify_on")
	backBtn := markup.Data("◀️ Назад", "back_from_alerts")

	markup.Inline(
		markup.Row(offBtn),
		markup.Row(maybeBtn),
		markup.Row(onBtn),
		markup.Row(backBtn),
	)

	return markup
}

// buildSettingsNotificationsFormatMarkup creates format settings keyboard with checkmarks for selected format
func (m *markups) buildSettingsNotificationsFormatMarkup(settings map[dal.SettingKey]interface{}) *tb.ReplyMarkup {
	markup := &tb.ReplyMarkup{}

	format, _ := settings[dal.SettingShutdownsMessageFormat].(string)

	linearText := "Лінійний"
	linearWithRangeText := "Лінійний (деталізований)"
	groupedText := "Згрупований"
	imageText := "Картинка"
	switch format {
	case "", dal.ShutdownsMessageFormatLinear:
		linearText = "✅ " + linearText
	case dal.ShutdownsMessageFormatLinearWithRange:
		linearWithRangeText = "✅ " + linearWithRangeText
	case dal.ShutdownsMessageFormatGrouped:
		groupedText = "✅ " + groupedText
	case dal.ShutdownsMessageFormatImage:
		imageText = "✅ " + imageText
	default:
		// unsupported
	}

	linearBtn := markup.Data(linearText, "set_format_linear")
	linearWithRangeBtn := markup.Data(linearWithRangeText, "set_format_linear_with_range")
	groupedBtn := markup.Data(groupedText, "set_format_grouped")
	imageBtn := markup.Data(imageText, "set_format_image")
	backBtn := markup.Data("◀️ Назад", "back_from_format")

	markup.Inline(
		markup.Row(linearBtn),
		markup.Row(linearWithRangeBtn),
		markup.Row(groupedBtn),
		markup.Row(imageBtn),
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
