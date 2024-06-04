package main

import (
	"log/slog"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/communication"
	"github.com/Roma7-7-7/sso-notifier/internal/service/shutdowns"
	"github.com/Roma7-7-7/sso-notifier/internal/service/subscription"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
)

func main() {
	store := dal.NewBoltDBStore("data/app.db")
	defer store.Close()

	bb := telegram.NewBotBuilder()

	subRepo := dal.NewSubscriptionRepo(store)
	shutdownsRepo := dal.NewShutdownsRepo(store)
	notificationRepo := dal.NewNotificationRepo(store)

	sender := bb.Sender(purgeSubscriber(subRepo))
	shutdownsService := shutdowns.NewShutdownsService(shutdownsRepo, providers.ChernivtsiShutdowns)
	notificationService := communication.NewNotificationService(notificationRepo, sender)
	subService := subscription.NewSubscriptionService(subRepo, shutdownsService, sender)

	scheduler := service.NewScheduler(shutdownsService, subService, notificationService)
	go scheduler.SendNotificationsTask()
	go scheduler.RefreshTable()
	go scheduler.SendUpdates()

	slog.Info("Starting bot")
	bb.Build(subService).Start()
}

func purgeSubscriber(subRepo subscription.Repository) func(chatID int64) {
	return func(chatID int64) {
		if err := subRepo.Purge(chatID); err != nil {
			slog.Error("failed to purge subscription", "chatID", chatID, "error", err)
		}
	}
}
