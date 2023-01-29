package main

import (
	"log"
	"testing"

	"go.uber.org/zap"
)

func Test_LoadTable(t *testing.T) {
	t.Skip()

	page, err := loadPage()
	if err != nil {
		t.Fatal(err)
	}

	table, err := parseShutdownsPage(page)
	if err != nil {
		t.Fatal(err)
	}

	group, err := renderGroup("1", table.Periods, table.Groups["1"].Items)
	if err != nil {
		t.Fatal(err)
	}
	log.Println(group)
}

func Test_QueueNotifyAll(t *testing.T) {
	t.Skip()

	store := NewBoltDBStore("data/app.db")
	subs, err := store.GetSubscribers()
	if err != nil {
		panic(err)
	}

	for _, s := range subs {
		if _, err = store.QueueNotification(s.ChatID, `Останні 2 дні бот не працював у зв'язку зі змінами на сайті Чернівціобленерго.
З мінусів: більше неможливо відправляти загальний графік одним фото.
З плюсів: можна підписатись на оновлення по групі (прнийамні до наступних змін на сайті).
Для вобору групи напишіть боту /start або /subscribe.`); err != nil {
			panic(err)
		}
	}

	zap.L().Info("Done")
}
