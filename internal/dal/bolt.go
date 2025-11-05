package dal

import (
	"fmt"
	"strconv"

	"go.etcd.io/bbolt"
)

type (
	BoltDB struct {
		db *bbolt.DB
	}
)

func NewBoltDB(db *bbolt.DB) (*BoltDB, error) {
	return &BoltDB{db: db}, nil
}

func (s *BoltDB) Purge(chatID int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		// Delete subscription
		subsBucket := tx.Bucket([]byte(subscriptionsBucket))
		if err := subsBucket.Delete(i64tob(chatID)); err != nil {
			return fmt.Errorf("delete subscriber with id=%d: %w", chatID, err)
		}

		prefix := fmt.Sprintf("%d_", chatID)

		// Delete all notification states for this user
		notifBucket := tx.Bucket([]byte(notificationsBucket))
		if notifBucket != nil {
			c := notifBucket.Cursor()
			for k, _ := c.Seek([]byte(prefix)); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == prefix; k, _ = c.Next() {
				if err := notifBucket.Delete(k); err != nil {
					return fmt.Errorf("delete notification state for key %s: %w", k, err)
				}
			}
		}

		// Delete all alerts for this user
		alertsBucket := tx.Bucket([]byte(alertsBucket))
		if alertsBucket != nil {
			c := alertsBucket.Cursor()
			for k, _ := c.Seek([]byte(prefix)); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == prefix; k, _ = c.Next() {
				if err := alertsBucket.Delete(k); err != nil {
					return fmt.Errorf("delete alert for key %s: %w", k, err)
				}
			}
		}

		return nil
	})
}

func (s *BoltDB) Close() error {
	return s.db.Close()
}

func i64tob(id int64) []byte {
	return []byte(strconv.FormatInt(id, 10))
}
