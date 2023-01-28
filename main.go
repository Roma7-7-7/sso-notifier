package main

import (
	"time"

	"go.uber.org/zap"
)

const notificationInterval = 5 * time.Minute

type Service interface {
	IsSubscribed(chatID int64) (bool, error)
	SetGroup(chatID int64, groupNum string) (Subscription, error)
	Unsubscribe(chatID int64) error
}

func main() {
	initLogger()

	store := NewBoltDBStore("data/app.db")
	defer store.Close()
	tbot := mustTBot()
	defer tbot.Close()
	sender := &tBotSender{tbot}
	service := NewCoreService(store, sender)

	go func() {
		for {
			service.SendQueuedNotifications()
			time.Sleep(notificationInterval)
		}
	}()

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
