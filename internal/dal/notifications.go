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
