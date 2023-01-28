package main

import (
	"time"

	"go.uber.org/zap"
)

const notificationInterval = 5 * time.Minute

type SubscriberStore interface {
	AddSubscriber(subscriber Subscriber) (bool, error)
	NumSubscribers() (int, error)
	PurgeSubscriber(s Subscriber) error

	QueueNotification(target Subscriber, msg string) (Notification, error)
	GetQueuedNotifications() ([]Notification, error)
	DeleteNotification(id int) error
}

type Sender interface {
	Send(s Subscriber, msg string) error
}

func main() {
	store := NewBoltDBStore("data/app.db")
	defer store.Close()
	tbot := mustTBot()
	defer tbot.Close()
	sender := &tBotSender{tbot}
	cs := NewCoreService(store, sender)

	go func() {
		for true {
			cs.SendQueuedNotifications()
			time.Sleep(notificationInterval)
		}
	}()

	zap.L().Info("Starting bot")
	NewBot(store, sender, tbot).Start()
}
