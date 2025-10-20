package dal

import (
	"encoding/json"
	"fmt"

	"go.etcd.io/bbolt"
)

type (
	Status string

	Shutdowns struct {
		Date    string                   `json:"date"`
		Periods []Period                 `json:"periods"`
		Groups  map[string]ShutdownGroup `json:"groups"`
	}

	Period struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	ShutdownGroup struct {
		Number int
		Items  []Status
	}

	Subscription struct {
		ChatID int64             `json:"chat_id"`
		Groups map[string]string `json:"groups"`
	}

	BoltDB struct {
		db *bbolt.DB
	}
)

const shutdownsBucket = "shutdowns"
const subscriptionsBucket = "subscriptions"

const shutdownsTableKey = "table"

const (
	ON    Status = "Y"
	OFF   Status = "N"
	MAYBE Status = "M"
)

func NewBoltDB(path string) (*BoltDB, error) {
	db, err := bbolt.Open(path, 0600, nil) //nolint:gomnd
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	mustBucket(db, shutdownsBucket)
	mustBucket(db, subscriptionsBucket)

	return &BoltDB{db: db}, nil
}

func (s *BoltDB) GetShutdowns() (Shutdowns, bool, error) {
	var res Shutdowns
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket([]byte(shutdownsBucket)).Get([]byte(shutdownsTableKey))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &res)
	})

	return res, found, err
}

func (s *BoltDB) PutShutdowns(t Shutdowns) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal shutdowns table: %w", err)
		}
		return tx.Bucket([]byte(shutdownsBucket)).Put([]byte(shutdownsTableKey), data)
	})
}

func (s *BoltDB) CountSubscriptions() (int, error) {
	var res int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

func (s *BoltDB) ExistsSubscription(chatID int64) (bool, error) {
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

func (s *BoltDB) GetSubscription(chatID int64) (Subscription, bool, error) {
	var res Subscription
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket([]byte(subscriptionsBucket)).Get(i64tob(chatID))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &res)
	})

	return res, found, err
}

func (s *BoltDB) GetAllSubscriptions() ([]Subscription, error) {
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

func (s *BoltDB) PutSubscription(sub Subscription) error {
	err := s.db.Update(func(tx *bbolt.Tx) error {
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

	return err
}

func (s *BoltDB) PurgeSubscriptions(chatID int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))

		if err := b.Delete(i64tob(chatID)); err != nil {
			return fmt.Errorf("failed to delete subscriber with id=%d: %w", chatID, err)
		}

		return nil
	})
}

func (s *BoltDB) Close() error {
	return s.db.Close()
}

func i64tob(id int64) []byte {
	return []byte(fmt.Sprintf("%d", id))
}

func mustBucket(db *bbolt.DB, name string) {
	if err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		return err
	}); err != nil {
		panic(fmt.Errorf("create bucket: %w", err))
	}
}
