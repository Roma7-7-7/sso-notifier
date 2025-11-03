package v4

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// SubscriptionV3 is the OLD structure (copy-pasted from dal/bolt.go as of v3)
// This represents subscriptions with mixed concerns: metadata + notification state
type SubscriptionV3 struct {
	ChatID    int64             `json:"chat_id"`
	Groups    map[string]string `json:"groups"` // group_id -> schedule_hash (MIXED CONCERN)
	CreatedAt time.Time         `json:"created_at"`
}

// SubscriptionV4 is the NEW structure with separated concerns
// Only stores subscription metadata, notification state moved to separate bucket
type SubscriptionV4 struct {
	ChatID    int64               `json:"chat_id"`
	Groups    map[string]struct{} `json:"groups"` // Set of group IDs (metadata only)
	CreatedAt time.Time           `json:"created_at"`
}

// NotificationStateV4 tracks what we last sent to a user for a specific date
// Separated from subscription metadata for cleaner architecture
type NotificationStateV4 struct {
	ChatID int64             `json:"chat_id"`
	Date   string            `json:"date"` // "2024-10-31" (YYYY-MM-DD format)
	SentAt time.Time         `json:"sent_at"`
	Hashes map[string]string `json:"hashes"` // group_id -> schedule_hash
}

// MigrationV4 splits subscription metadata from notification state
type MigrationV4 struct{}

// Version returns the migration version
func (m *MigrationV4) Version() int {
	return 4 //nolint:mnd // version 4
}

// Description returns a human-readable description of the migration
func (m *MigrationV4) Description() string {
	return "Split subscription metadata from notification state"
}

// Up performs the migration
func (m *MigrationV4) Up(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		// 1. Create notifications bucket
		notifBucket, err := tx.CreateBucketIfNotExists([]byte("notifications"))
		if err != nil {
			return fmt.Errorf("create notifications bucket: %w", err)
		}

		// 2. Get subscriptions bucket
		subsBucket := tx.Bucket([]byte("subscriptions"))
		if subsBucket == nil {
			// No subscriptions bucket means nothing to migrate
			return nil
		}

		c := subsBucket.Cursor()
		now := time.Now()
		today := now.Format("2006-01-02") // YYYY-MM-DD
		migratedCount := 0

		// 3. Migrate each subscription
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// Unmarshal old structure
			var oldSub SubscriptionV3
			if err := json.Unmarshal(v, &oldSub); err != nil {
				return fmt.Errorf("unmarshal v3 subscription for key %s: %w", k, err)
			}

			// 4. Transform subscription: extract group numbers into set
			newSub := SubscriptionV4{
				ChatID:    oldSub.ChatID,
				Groups:    make(map[string]struct{}),
				CreatedAt: oldSub.CreatedAt,
			}

			// 5. Extract hashes for notification state
			hashes := make(map[string]string)
			for groupNum, hash := range oldSub.Groups {
				// Add group to subscription set
				newSub.Groups[groupNum] = struct{}{}

				// Only store hash if it exists (not empty string)
				if hash != "" {
					hashes[groupNum] = hash
				}
			}

			// 6. Write new subscription structure
			newSubData, err := json.Marshal(newSub)
			if err != nil {
				return fmt.Errorf("marshal v4 subscription for chatID=%d: %w", newSub.ChatID, err)
			}

			if err := subsBucket.Put(k, newSubData); err != nil {
				return fmt.Errorf("put v4 subscription for chatID=%d: %w", newSub.ChatID, err)
			}

			// 7. Create notification state (only if we have hashes)
			if len(hashes) > 0 {
				notifState := NotificationStateV4{
					ChatID: oldSub.ChatID,
					Date:   today,
					SentAt: now, // Assume last notification was sent at migration time
					Hashes: hashes,
				}

				notifData, err := json.Marshal(notifState)
				if err != nil {
					return fmt.Errorf("marshal notification state for chatID=%d: %w", oldSub.ChatID, err)
				}

				// Key format: <chatID>_<YYYY-MM-DD>
				notifKey := fmt.Sprintf("%d_%s", oldSub.ChatID, today)
				if err := notifBucket.Put([]byte(notifKey), notifData); err != nil {
					return fmt.Errorf("put notification state for chatID=%d: %w", oldSub.ChatID, err)
				}
			}

			migratedCount++
		}

		// Note: migratedCount would be useful for logging, but we don't have logger access here
		_ = migratedCount

		return nil
	})
}

// New creates a new instance of MigrationV4
func New() *MigrationV4 {
	return &MigrationV4{}
}
