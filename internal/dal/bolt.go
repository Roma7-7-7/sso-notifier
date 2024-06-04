package dal

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"

	"go.etcd.io/bbolt"

	"github.com/Roma7-7-7/sso-notifier/models"
)

const shutdownsBucket = "shutdowns"
const subscriptionsBucket = "subscriptions"
const notificationsBucket = "notifications"

type BoltDBStore struct {
	db *bbolt.DB
}

func (s *BoltDBStore) SubscriptionsSize() (int, error) {
	var res int
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

func (s *BoltDBStore) SubscriptionExists(chatID int64) (bool, error) {
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

func (s *BoltDBStore) SubscriptionGet(chatID int64) (models.Subscription, bool, error) {
	var res models.Subscription
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

func (s *BoltDBStore) SubscriptionGetAll() ([]models.Subscription, error) {
	var res []models.Subscription

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(subscriptionsBucket))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var sub models.Subscription
			if err := json.Unmarshal(v, &sub); err != nil {
				return fmt.Errorf("failed to unmarshal subscription: %w", err)
			}
			res = append(res, sub)
		}

		return nil
	})

	return res, err
}

func (s *BoltDBStore) SubscriptionPut(sub models.Subscription) (models.Subscription, error) {
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

	return sub, err
}

func (s *BoltDBStore) SubscriptionPurge(chatID int64) error {
	ns, err := s.NotificationGetAll()
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

func (s *BoltDBStore) ShutdownsTableGet(key string) (models.ShutdownsTable, bool, error) {
	var res models.ShutdownsTable
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket([]byte(shutdownsBucket)).Get([]byte(key))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &res)
	})

	return res, found, err
}

func (s *BoltDBStore) ShutdownsTablePut(t models.ShutdownsTable) (models.ShutdownsTable, error) {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("failed to marshal shutdowns table: %w", err)
		}
		return tx.Bucket([]byte(shutdownsBucket)).Put([]byte(t.ID), data)
	})

	return t, err
}

func (s *BoltDBStore) NotificationGetAll() ([]models.Notification, error) {
	res := make([]models.Notification, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(notificationsBucket)).Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var n models.Notification
			if err := json.Unmarshal(v, &n); err != nil {
				return fmt.Errorf("failed to unmarshal notification: %w", err)
			}
			res = append(res, n)
		}
		return nil

	})
	return res, err
}

func (s *BoltDBStore) NotificationPut(n models.Notification) (models.Notification, error) {
	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		id, _ := b.NextSequence() //nolint:errcheck
		n.ID = int(id)
		bytes, err := json.Marshal(n)
		if err != nil {
			return fmt.Errorf("failed to marshal notification: %w", err)
		}

		return b.Put(itob(n.ID), bytes)
	})
	return n, err
}

func (s *BoltDBStore) NotificationDelete(id int) error {
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

func NewBoltDBStore(path string) *BoltDBStore {
	db, err := bbolt.Open(path, 0600, nil) //nolint:gomnd
	if err != nil {
		slog.Error("failed to open bolt db", "error", err, "path", path)
		panic(fmt.Errorf("open bolt db: %w", err))
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
		slog.Error("failed to create bucket", "name", name, "error", err)
		panic(fmt.Errorf("create bucket: %w", err))
	}
}

type SubscriptionBoltDBRepo struct {
	delegate *BoltDBStore
}

func (r *SubscriptionBoltDBRepo) Size() (int, error) {
	return r.delegate.SubscriptionsSize()
}

func (r *SubscriptionBoltDBRepo) Exists(chatID int64) (bool, error) {
	return r.delegate.SubscriptionExists(chatID)
}

func (r *SubscriptionBoltDBRepo) Get(chatID int64) (models.Subscription, bool, error) {
	return r.delegate.SubscriptionGet(chatID)
}

func (r *SubscriptionBoltDBRepo) GetAll() ([]models.Subscription, error) {
	return r.delegate.SubscriptionGetAll()
}

func (r *SubscriptionBoltDBRepo) Put(sub models.Subscription) (models.Subscription, error) {
	return r.delegate.SubscriptionPut(sub)
}

func (r *SubscriptionBoltDBRepo) Purge(chatID int64) error {
	return r.delegate.SubscriptionPurge(chatID)
}

func NewSubscriptionRepo(delegate *BoltDBStore) *SubscriptionBoltDBRepo {
	return &SubscriptionBoltDBRepo{delegate: delegate}
}

type ShutdownBoltsDBRepo struct {
	delegate *BoltDBStore
}

func (r *ShutdownBoltsDBRepo) Get(id string) (models.ShutdownsTable, bool, error) {
	return r.delegate.ShutdownsTableGet(id)
}

func (r *ShutdownBoltsDBRepo) Put(t models.ShutdownsTable) (models.ShutdownsTable, error) {
	return r.delegate.ShutdownsTablePut(t)
}

func NewShutdownsRepo(delegate *BoltDBStore) *ShutdownBoltsDBRepo {
	return &ShutdownBoltsDBRepo{delegate: delegate}
}

type NotificationRepo struct {
	delegate *BoltDBStore
}

func (r *NotificationRepo) GetAll() ([]models.Notification, error) {
	return r.delegate.NotificationGetAll()
}

func (r *NotificationRepo) Put(n models.Notification) (models.Notification, error) {
	return r.delegate.NotificationPut(n)
}

func (r *NotificationRepo) Delete(id int) error {
	return r.delegate.NotificationDelete(id)
}

func NewNotificationRepo(delegate *BoltDBStore) *NotificationRepo {
	return &NotificationRepo{delegate: delegate}
}
