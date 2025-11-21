package dal

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

const notificationsBucket = "notifications"

type NotificationState struct {
	ChatID int64             `json:"chat_id"`
	Date   string            `json:"date"`
	SentAt time.Time         `json:"sent_at"`
	Hashes map[string]string `json:"hashes"`
}

// CountNotificationStates total number of notification states
func (s *BoltDB) CountNotificationStates() (int, error) {
	res := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		res = b.Stats().KeyN
		return nil
	})
	return res, err
}

// GetNotificationState retrieves notification state for a specific user and date
func (s *BoltDB) GetNotificationState(chatID int64, date Date) (NotificationState, bool, error) {
	var res NotificationState
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		if b == nil {
			return nil
		}

		key := notificationKey(chatID, date.ToKey())
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}

		found = true
		return json.Unmarshal(data, &res)
	})

	return res, found, err
}

// PutNotificationState stores notification state for a specific user and date
func (s *BoltDB) PutNotificationState(state NotificationState) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		if b == nil {
			return errors.New("notifications bucket not found")
		}

		key := notificationKey(state.ChatID, state.Date)
		data, err := json.Marshal(&state)
		if err != nil {
			return fmt.Errorf("marshal notification state for chatID=%d date=%s: %w", state.ChatID, state.Date, err)
		}

		if err := b.Put([]byte(key), data); err != nil {
			return fmt.Errorf("put notification state for chatID=%d date=%s: %w", state.ChatID, state.Date, err)
		}

		return nil
	})
}

// DeleteNotificationStates removes all notification states for a specific user
func (s *BoltDB) DeleteNotificationStates(chatID int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return s.deleteNotificationStates(tx, chatID)
	})
}

// CleanupNotificationStates remove notification states older than passed TTL
func (s *BoltDB) CleanupNotificationStates(olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		return b.ForEach(func(k, v []byte) error {
			var state NotificationState
			if err := json.Unmarshal(v, &state); err != nil {
				return fmt.Errorf("unmarshal notification state for chatID=%d: %w", state.ChatID, err)
			}
			if state.SentAt.IsZero() {
				return nil
			}
			if state.SentAt.After(s.clock.Now().Add(-olderThan)) {
				return nil
			}
			return b.Delete(k)
		})
	})
}

func (s *BoltDB) deleteNotificationStates(tx *bbolt.Tx, chatID int64) error {
	b := tx.Bucket([]byte(notificationsBucket))

	prefix := fmt.Sprintf("%d_", chatID)
	c := b.Cursor()

	// Find and delete all keys with this chatID prefix
	for k, _ := c.Seek([]byte(prefix)); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == prefix; k, _ = c.Next() {
		if err := b.Delete(k); err != nil {
			return fmt.Errorf("delete notification state for key %s: %w", k, err)
		}
	}

	return nil
}

func notificationKey(chatID int64, date string) string {
	return fmt.Sprintf("%d_%s", chatID, date)
}
