package dal

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

const subscriptionsBucket = "subscriptions"

type Subscription struct {
	ChatID    int64                      `json:"chat_id"`
	Groups    map[string]struct{}        `json:"groups"`
	CreatedAt time.Time                  `json:"created_at"`
	Settings  map[SettingKey]interface{} `json:"settings,omitempty"` // nil by default, backward compatible
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
				return fmt.Errorf("unmarshal subscription: %w", err)
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

		existing, exists, err := s.GetSubscription(sub.ChatID)
		if err != nil {
			return fmt.Errorf("get existing subscription: %w", err)
		}

		if !exists {
			sub.CreatedAt = s.now()
		} else {
			// make sure we do not override created at
			sub.CreatedAt = existing.CreatedAt
		}

		id := i64tob(sub.ChatID)
		data, err := json.Marshal(&sub)
		if err != nil {
			return fmt.Errorf("marshal subscription for chatID=%d: %w", sub.ChatID, err)
		}
		if err := b.Put(id, data); err != nil {
			return fmt.Errorf("put subscription for chatID=%d: %w", sub.ChatID, err)
		}

		return nil
	})

	return err
}

// GetBoolSetting retrieves a boolean setting from the settings map with a default value
func GetBoolSetting(settings map[SettingKey]interface{}, key SettingKey, defaultValue bool) bool {
	if settings == nil {
		return defaultValue
	}

	val, exists := settings[key]
	if !exists {
		return defaultValue
	}

	boolVal, ok := val.(bool)
	if !ok {
		return defaultValue
	}

	return boolVal
}

// GetIntSetting retrieves an integer setting from the settings map with a default value
func GetIntSetting(settings map[SettingKey]interface{}, key SettingKey, defaultValue int) int {
	if settings == nil {
		return defaultValue
	}

	val, exists := settings[key]
	if !exists {
		return defaultValue
	}

	// Handle both int and float64 (JSON unmarshaling uses float64 for numbers)
	switch v := val.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	default:
		return defaultValue
	}
}

// GetStringSetting retrieves a string setting from the settings map with a default value
func GetStringSetting(settings map[string]interface{}, key SettingKey, defaultValue string) string {
	if settings == nil {
		return defaultValue
	}

	val, exists := settings[string(key)]
	if !exists {
		return defaultValue
	}

	strVal, ok := val.(string)
	if !ok {
		return defaultValue
	}

	return strVal
}
