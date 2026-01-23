package dal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.etcd.io/bbolt"
)

const (
	emergencyStateKey              = "_emergency"
	emergencyNotificationKeyPrefix = "EMERGENCY_"
)

// EmergencyState represents the global emergency mode state.
type EmergencyState struct {
	Active    bool      `json:"active"`
	StartedAt time.Time `json:"started_at"`
}

// GetEmergencyState retrieves the current emergency state from storage.
func (s *BoltDB) GetEmergencyState() (EmergencyState, error) {
	var state EmergencyState

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(shutdownsBucket))
		if b == nil {
			return nil
		}

		data := b.Get([]byte(emergencyStateKey))
		if data == nil {
			return nil
		}

		return json.Unmarshal(data, &state)
	})

	return state, err
}

// SetEmergencyState updates the emergency state in storage.
func (s *BoltDB) SetEmergencyState(state EmergencyState) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(shutdownsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", shutdownsBucket)
		}

		data, err := json.Marshal(&state)
		if err != nil {
			return fmt.Errorf("marshal emergency state: %w", err)
		}

		return b.Put([]byte(emergencyStateKey), data)
	})
}

// GetEmergencyNotification checks if a user was notified about the current emergency.
// Returns the notification time, whether the user was notified, and any error.
func (s *BoltDB) GetEmergencyNotification(chatID int64) (time.Time, bool, error) {
	var sentAt time.Time
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		if b == nil {
			return nil
		}

		key := emergencyNotificationKey(chatID)
		data := b.Get([]byte(key))
		if data == nil {
			return nil
		}

		found = true
		return json.Unmarshal(data, &sentAt)
	})

	return sentAt, found, err
}

// SetEmergencyNotification marks a user as notified about the emergency.
func (s *BoltDB) SetEmergencyNotification(chatID int64, sentAt time.Time) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", notificationsBucket)
		}

		key := emergencyNotificationKey(chatID)
		data, err := json.Marshal(sentAt)
		if err != nil {
			return fmt.Errorf("marshal emergency notification time: %w", err)
		}

		return b.Put([]byte(key), data)
	})
}

// ClearAllEmergencyNotifications removes all emergency notification records.
// Called when emergency ends to reset state for the next emergency.
func (s *BoltDB) ClearAllEmergencyNotifications() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(notificationsBucket))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, _ := c.Seek([]byte(emergencyNotificationKeyPrefix)); k != nil && strings.HasPrefix(string(k), emergencyNotificationKeyPrefix); k, _ = c.Next() {
			if err := b.Delete(k); err != nil {
				return fmt.Errorf("delete emergency notification key %s: %w", k, err)
			}
		}

		return nil
	})
}

// deleteEmergencyNotification removes the emergency notification for a specific user.
func (s *BoltDB) deleteEmergencyNotification(tx *bbolt.Tx, chatID int64) error {
	b := tx.Bucket([]byte(notificationsBucket))
	if b == nil {
		return nil
	}

	key := emergencyNotificationKey(chatID)
	return b.Delete([]byte(key))
}

func emergencyNotificationKey(chatID int64) string {
	return fmt.Sprintf("%s%d", emergencyNotificationKeyPrefix, chatID)
}
