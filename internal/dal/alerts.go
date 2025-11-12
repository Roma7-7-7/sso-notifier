package dal

import (
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

const alertsBucket = "alerts"

type AlertKey string

// BuildAlertKey creates a key for the alerts bucket
func BuildAlertKey(chatID int64, date, timeStr, status, group string) AlertKey {
	return AlertKey(fmt.Sprintf("%d_%s_%s_%s_%s", chatID, date, timeStr, status, group))
}

type SettingKey string

const (
	SettingNotifyOn    SettingKey = "notify_on_10min"
	SettingNotifyOff   SettingKey = "notify_off_10min"
	SettingNotifyMaybe SettingKey = "notify_maybe_10min"
)

// GetAlert checks if an alert was already sent for the given key
func (s *BoltDB) GetAlert(key AlertKey) (time.Time, bool, error) {
	var sentAt time.Time
	found := false
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(alertsBucket))
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}

		found = true
		var parseErr error
		sentAt, parseErr = time.Parse(time.RFC3339, string(data))
		if parseErr != nil {
			return fmt.Errorf("parse data: %w", parseErr)
		}
		return nil
	})

	return sentAt, found, err
}

// PutAlert records that an alert was sent at the given time
func (s *BoltDB) PutAlert(key AlertKey, sentAt time.Time) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(alertsBucket))
		timestamp := []byte(sentAt.Format(time.RFC3339))
		if err := b.Put([]byte(key), timestamp); err != nil {
			return fmt.Errorf("put alert for key %s: %w", key, err)
		}

		return nil
	})
}

// DeleteAlert removes a single alert record
func (s *BoltDB) DeleteAlert(key AlertKey) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(alertsBucket))
		if err := b.Delete([]byte(key)); err != nil {
			return fmt.Errorf("delete alert for key %s: %w", key, err)
		}

		return nil
	})
}

// DeleteAlerts removes all alert records for a specific user
func (s *BoltDB) DeleteAlerts(chatID int64) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return s.deleteAlerts(tx, chatID)
	})
}

func (s *BoltDB) deleteAlerts(tx *bbolt.Tx, chatID int64) error {
	b := tx.Bucket([]byte(alertsBucket))
	prefix := fmt.Sprintf("%d_", chatID)
	c := b.Cursor()

	// Find and delete all keys with this chatID prefix
	for k, _ := c.Seek([]byte(prefix)); k != nil && len(k) >= len(prefix) && string(k[:len(prefix)]) == prefix; k, _ = c.Next() {
		if err := b.Delete(k); err != nil {
			return fmt.Errorf("delete alert for key %s: %w", k, err)
		}
	}

	return nil
}
