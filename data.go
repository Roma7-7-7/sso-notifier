package main

import (
	"fmt"
	"log"
	"strconv"
	"sync"

	"go.etcd.io/bbolt"
	tele "gopkg.in/telebot.v3"
)

const subscribersBucket = "subscribers"

type BoltDBStore struct {
	db *bbolt.DB
}

func (s *BoltDBStore) AddSubscriber(c tele.Context) bool {
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
	if err != nil {
		log.Printf("failed to add subscriber: %v", err)
		return false
	}

	return res
}

func (s *BoltDBStore) GetWithDifferentHash(hash string) []tele.ChatID {
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
	if err != nil {
		log.Printf("failed to get subscribers: %v", err)
		return nil
	}

	return res
}

func (s *BoltDBStore) UpdateHash(id tele.ChatID, hash string) {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		return b.Put(idToBytes(int64(id)), []byte(hash))
	})
	if err != nil {
		log.Printf("failed to update hash: %v", err)
	}
}

func (s *BoltDBStore) Size() int {
	var res int

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		res = b.Stats().KeyN
		return nil
	})
	if err != nil {
		log.Printf("failed to get number subscribers: %v", err)
		return 0
	}

	return res
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

type InMemoryStore struct {
	subscribers map[tele.ChatID]string

	mx sync.Mutex
}

func (s *InMemoryStore) AddSubscriber(c tele.Context) bool {
	s.mx.Lock()
	defer s.mx.Unlock()
	if _, ok := s.subscribers[tele.ChatID(c.Chat().ID)]; ok {
		return false
	}

	s.subscribers[tele.ChatID(c.Chat().ID)] = ""
	return true
}

func (s *InMemoryStore) GetWithDifferentHash(hash string) []tele.ChatID {
	s.mx.Lock()
	defer s.mx.Unlock()

	res := make([]tele.ChatID, 0)
	for k, v := range s.subscribers {
		if v != hash {
			res = append(res, k)
		}
	}
	return res
}

func (s *InMemoryStore) UpdateHash(id tele.ChatID, hash string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.subscribers[id] = hash
}

func (s *InMemoryStore) Size() int {
	s.mx.Lock()
	defer s.mx.Unlock()
	return len(s.subscribers)
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		subscribers: make(map[tele.ChatID]string),
	}
}
