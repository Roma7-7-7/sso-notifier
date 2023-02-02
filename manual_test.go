package main

import (
	"log"
	"testing"

	"go.uber.org/zap"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
	"github.com/Roma7-7-7/sso-notifier/models"
)

func Test_LoadTable(t *testing.T) {
	t.Skip()

	table, _ := providers.ChernivtsiShutdowns()
	log.Println(table)

	// group, err := subscription.renderGroup("1", table.Periods, table.Groups["1"].Items)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// log.Println(group)
}

func Test_QueueNotifyAll(t *testing.T) {
	t.Skip()

	store := dal.NewBoltDBStore("data/app.db")
	subs, err := store.SubscriptionGetAll()
	if err != nil {
		panic(err)
	}

	for _, s := range subs {
		msg := `Останні 2 дні бот не працював у зв'язку зі змінами на сайті Чернівціобленерго.
З мінусів: більше неможливо відправляти загальний графік одним фото.
З плюсів: можна підписатись на оновлення по групі (прнийамні до наступних змін на сайті).
Для вобору групи напишіть боту /start або /subscribe.`
		if _, err = store.NotificationPut(models.Notification{
			Target: s.ChatID,
			Msg:    msg,
		}); err != nil {
			panic(err)
		}
	}

	zap.L().Info("Done")
}
