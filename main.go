package main

import (
	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/internal/service"
	"github.com/Roma7-7-7/sso-notifier/internal/service/communication"
	"github.com/Roma7-7-7/sso-notifier/internal/service/shutdowns"
	"github.com/Roma7-7-7/sso-notifier/internal/service/subscription"
	"github.com/Roma7-7-7/sso-notifier/internal/telegram"
	"go.uber.org/zap"
)

func main() {
	initLogger()

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

	zap.L().Info("Starting bot")
	bb.Build(subService).Start()
}

func purgeSubscriber(subRepo subscription.Repository) func(chatID int64) {
	return func(chatID int64) {
		if err := subRepo.Purge(chatID); err != nil {
			zap.L().Error("failed to purge subscription", zap.Int64("chatID", chatID), zap.Error(err))
		}
	}
}

func initLogger() {
	zapCfg := zap.NewDevelopmentConfig()
	zapCfg.Level.SetLevel(zap.DebugLevel)
	logger, err := zapCfg.Build()
	if err != nil {
		panic(err)
	}
	zap.ReplaceGlobals(logger)
}
