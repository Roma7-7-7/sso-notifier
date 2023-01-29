package main

import (
	"encoding/json"
	"fmt"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"log"
	"testing"
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

func Test_MigrateSubscriptions(t *testing.T) {
	t.Skip()

	db, err := bbolt.Open("data/app.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("subscribers"))
		_, err := tx.CreateBucketIfNotExists([]byte("subscriptions"))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		target := tx.Bucket([]byte("subscriptions"))

		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			sub := Subscription{
				ChatID: btoi64(k),
				Groups: make(map[string]string),
			}
			data, err := json.Marshal(&sub)
			if err != nil {
				return fmt.Errorf("failed to marshal subscription: %w", err)
			}

			if err := target.Put(i64tob(sub.ChatID), data); err != nil {
				return fmt.Errorf("failed to put subscription: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		panic(err)
	}
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
