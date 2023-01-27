package main

import (
	"fmt"
	"go.etcd.io/bbolt"
	tele "gopkg.in/telebot.v3"
	"log"
	"strconv"
)

const subscribersBucket = "subscribers"

type BoltDBStore struct {
	db *bbolt.DB
}

func (s *BoltDBStore) AddSubscriber(c tele.Context) (bool, error) {
	res := false
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		id := idToBytes(c.Chat().ID)
		if b.Get(id) != nil {
			return nil
		}

		if err := b.Put(id, []byte("")); err != nil {
			return err
		}

		res = true
		return nil
	})
	return res, err
}

func (s *BoltDBStore) GetWithDifferentHash(hash string) ([]tele.ChatID, error) {
	res := make([]tele.ChatID, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if string(v) != hash {
				res = append(res, bytesToID(k))
			}
		}
		return nil
	})
	return res, err
}

func (s *BoltDBStore) DeleteByChatID(id tele.ChatID) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		return b.Delete(idToBytes(int64(id)))
	})
}

func (s *BoltDBStore) UpdateHash(id tele.ChatID, hash string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		return b.Put(idToBytes(int64(id)), []byte(hash))
	})
}

func (s *BoltDBStore) Size() (int, error) {
	var res int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

func idToBytes(id int64) []byte {
	return []byte(fmt.Sprintf("%d", id))
}

func bytesToID(b []byte) tele.ChatID {
	id, _ := strconv.ParseInt(string(b), 10, 64) //nolint:errcheck
	return tele.ChatID(id)
}

func NewBoltDBStore(path string) *BoltDBStore {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		log.Fatalf("failed to open bolt db: %v", err)
	}
	if err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(subscribersBucket))
		return err
	}); err != nil {
		log.Fatalf("failed to create subscribers bucket: %v", err)
	}
	return &BoltDBStore{db: db}
}
