package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const shutdownsTableKey = "table"
const shutdownsBucket = "shutdowns"
const subscriptionsBucket = "subscriptions"
const notificationsBucket = "notifications"

var ErrNotFound = fmt.Errorf("not found")

type BoltDBStore struct {
	db *bbolt.DB
}

func (s *BoltDBStore) IsSubscribed(chatID int64) (bool, error) {
	res := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		if b.Get(i64tob(chatID)) != nil {
			res = true
		}
		return nil
	})

	return res, err
}

func (s *BoltDBStore) GetSubscriptions() ([]Subscription, error) {
	var res []Subscription

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var sub Subscription
			if err := json.Unmarshal(v, &sub); err != nil {
				return fmt.Errorf("failed to unmarshal subscription: %w", err)
			}
			res = append(res, sub)
		}

		return nil
	})

	return res, err
}

func (s *BoltDBStore) SetSubscription(chatID int64, groupNum string) (Subscription, error) {
	res := Subscription{
		ChatID: chatID,
		Groups: map[string]string{
			groupNum: "",
		},
	}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		var err error
		b := tx.Bucket([]byte(subscriptionsBucket))

		id := i64tob(chatID)
		var data []byte
		if data, err = json.Marshal(&res); err != nil {
			return fmt.Errorf("failed to marshal subscription for chatID=%d: %w", chatID, err)
		}
		if err := b.Put(id, data); err != nil {
			return fmt.Errorf("failed to put subscription for chatID=%d: %w", chatID, err)
		}

		return nil
	})

	return res, err
}

func (s *BoltDBStore) UpdateSubscription(sub Subscription) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))

		id := i64tob(sub.ChatID)
		data, err := json.Marshal(&sub)
		if err != nil {
			return fmt.Errorf("failed to marshal subscription for chatID=%d: %w", sub.ChatID, err)
		}
		if err := b.Put(id, data); err != nil {
			return fmt.Errorf("failed to put subscription for chatID=%d: %w", sub.ChatID, err)
		}

		return nil
	})
}

func (s *BoltDBStore) PurgeSubscriptions(chatID int64) error {
	ns, err := s.GetQueuedNotifications()
	if err != nil {
		return fmt.Errorf("failed to get queued notifications: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))

		if err := b.Delete(i64tob(chatID)); err != nil {
			return fmt.Errorf("failed to delete subscriber with id=%d: %w", chatID, err)
		}

		b = tx.Bucket([]byte(notificationsBucket))
		for _, n := range ns {
			if n.Target != chatID {
				continue
			}

			if err := b.Delete(itob(n.ID)); err != nil {
				return fmt.Errorf("failed to delete notification with id=%d: %w", n.ID, err)
			}
		}

		return nil
	})
}

func (s *BoltDBStore) GetSubscribers() ([]Subscription, error) {
	res := make([]Subscription, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(subscriptionsBucket)).Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			res = append(res, Subscription{
				ChatID: btoi64(k),
			})
		}

		return nil
	})
	return res, err
}

func (s *BoltDBStore) NumSubscribers() (int, error) {
	var res int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

func (s *BoltDBStore) UpdateShutdownsTable(t ShutdownsTable) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("failed to marshal shutdowns table: %w", err)
		}
		return tx.Bucket([]byte(shutdownsBucket)).Put([]byte(shutdownsTableKey), data)
	})
}

func (s *BoltDBStore) GetShutdownsTable() (ShutdownsTable, error) {
	var res ShutdownsTable
	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket([]byte(shutdownsBucket)).Get([]byte(shutdownsTableKey))
		if data == nil {
			return ErrNotFound
		}

		if err := json.Unmarshal(data, &res); err != nil {
			return fmt.Errorf("failed to unmarshal shutdowns table: %w", err)
		}

		return nil
	})
	return res, err
}

func (s *BoltDBStore) QueueNotification(target int64, msg string) (Notification, error) {
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
	b := make([]byte, 8) //nolint:gomnd
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func i64tob(id int64) []byte {
	return []byte(fmt.Sprintf("%d", id))
}

func btoi64(b []byte) int64 {
	id, _ := strconv.ParseInt(string(b), 10, 64) //nolint:errcheck
	return id
}

func NewBoltDBStore(path string) *BoltDBStore {
	db, err := bbolt.Open(path, 0600, nil) //nolint:gomnd
	if err != nil {
		zap.L().Fatal("failed to open bolt db", zap.Error(err))
	}

	mustBucket(db, shutdownsBucket)
	mustBucket(db, subscriptionsBucket)
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
