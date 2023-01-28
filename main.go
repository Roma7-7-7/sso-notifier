package main

import (
	"go.uber.org/zap"
)

type Service interface {
	IsSubscribed(chatID int64) (bool, error)
	GetSubscriptions() ([]Subscription, error)
	SubscribeToGroup(chatID int64, groupNum string) (Subscription, error)
	UpdateSubscription(sub Subscription) error
	Unsubscribe(chatID int64) error

	UpdateShutdownsTable(st ShutdownsTable) error
	GetShutdownsTable() (ShutdownsTable, bool, error)

	SendQueuedNotifications()
}

func main() {
	initLogger()

	store := NewBoltDBStore("data/app.db")
	defer store.Close()
	tbot := mustTBot()
	defer tbot.Close()
	sender := &tBotSender{tbot}
	service := NewCoreService(store, sender)

	scheduler := NewScheduler(service, sender)
	go scheduler.SendNotificationsTask()
	go scheduler.RefreshTable()
	go scheduler.SendUpdates()

	zap.L().Info("Starting bot")
	NewBot(service, tbot).Start()
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
