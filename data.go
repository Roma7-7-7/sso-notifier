package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const subscribersBucket = "subscribers"
const notificationsBucket = "notifications"

type BoltDBStore struct {
	db *bbolt.DB
}

func (s *BoltDBStore) AddSubscriber(sub Subscriber) (bool, error) {
	res := false
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		id := i64tob(sub.ChatID)
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

func (s *BoltDBStore) PurgeSubscriber(sub Subscriber) error {
	ns, err := s.GetQueuedNotifications()
	if err != nil {
		return fmt.Errorf("failed to get queued notifications: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))

		if err := b.Delete(i64tob(sub.ChatID)); err != nil {
			return fmt.Errorf("failed to delete subscriber with id=%d: %w", sub.ChatID, err)
		}

		b = tx.Bucket([]byte(notificationsBucket))
		for _, n := range ns {
			if n.Target.ChatID != sub.ChatID {
				continue
			}

			if err := b.Delete(itob(n.ID)); err != nil {
				return fmt.Errorf("failed to delete notification with id=%d: %w", n.ID, err)
			}
		}

		return nil
	})
}

func (s *BoltDBStore) NumSubscribers() (int, error) {
	var res int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscribersBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

func (s *BoltDBStore) QueueNotification(target Subscriber, msg string) (Notification, error) {
	var res Notification
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		id, _ := b.NextSequence() //nolint:errcheck
		res = Notification{
			ID:     int(id),
			Target: target,
			Msg:    msg,
		}
		bytes, err := json.Marshal(res)
		if err != nil {
			return fmt.Errorf("failed to marshal notification: %w", err)
		}

		return b.Put(itob(res.ID), bytes)
	})
	return res, err
}

func (s *BoltDBStore) GetQueuedNotifications() ([]Notification, error) {
	res := make([]Notification, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(notificationsBucket)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var n Notification
			if err := json.Unmarshal(v, &n); err != nil {
				return fmt.Errorf("failed to unmarshal notification: %w", err)
			}
			res = append(res, n)
		}
		return nil

	})
	return res, err
}

func (s *BoltDBStore) DeleteNotification(id int) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		return b.Delete(itob(id))
	})
}

func (s *BoltDBStore) Close() error {
	return s.db.Close()
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func i64tob(id int64) []byte {
	return []byte(fmt.Sprintf("%d", id))
}

func NewBoltDBStore(path string) *BoltDBStore {
	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		zap.L().Fatal("failed to open bolt db", zap.Error(err))
	}

	mustBucket(db, subscribersBucket)
	mustBucket(db, notificationsBucket)

	return &BoltDBStore{db: db}
}

func mustBucket(db *bbolt.DB, name string) {
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		return err
	}); err != nil {
		zap.L().Fatal("failed to create bucket", zap.String("name", name), zap.Error(err))
	}
}
