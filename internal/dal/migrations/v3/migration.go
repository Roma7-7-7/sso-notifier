package v3

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

// SubscriptionV2 is the OLD structure (copy-pasted from dal/bolt.go as of v2)
// This represents subscriptions before the CreatedAt field was added
type SubscriptionV2 struct {
	ChatID int64             `json:"chat_id"`
	Groups map[string]string `json:"groups"`
}

// SubscriptionV3 is the NEW structure with CreatedAt field
type SubscriptionV3 struct {
	ChatID    int64             `json:"chat_id"`
	Groups    map[string]string `json:"groups"`
	CreatedAt time.Time         `json:"created_at"`
}

// MigrationV3 adds CreatedAt timestamp to all subscriptions
type MigrationV3 struct{}

// Version returns the migration version
func (m *MigrationV3) Version() int {
	return 3 //nolint:mnd // version 3
}

// Description returns a human-readable description of the migration
func (m *MigrationV3) Description() string {
	return "Add CreatedAt timestamp to subscriptions"
}

// Up performs the migration
func (m *MigrationV3) Up(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("subscriptions"))
		if b == nil {
			// No subscriptions bucket means no subscriptions to migrate
			// This shouldn't happen as v2 creates the bucket, but handle gracefully
			return nil
		}

		c := b.Cursor()
		now := time.Now()
		migratedCount := 0

		// Iterate over all subscriptions
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// Unmarshal old structure
			var oldSub SubscriptionV2
			if err := json.Unmarshal(v, &oldSub); err != nil {
				return fmt.Errorf("unmarshal old subscription for key %s: %w", k, err)
			}

			// Check if already migrated (has CreatedAt field)
			// We do this by trying to unmarshal as V3 and checking if CreatedAt is zero
			var checkSub SubscriptionV3
			if err := json.Unmarshal(v, &checkSub); err == nil && !checkSub.CreatedAt.IsZero() {
				// Already has CreatedAt, skip
				continue
			}

			// Transform to new structure
			newSub := SubscriptionV3{
				ChatID:    oldSub.ChatID,
				Groups:    oldSub.Groups,
				CreatedAt: now, // Set to migration time for all existing subscriptions
			}

			// Marshal and write back
			newData, err := json.Marshal(newSub)
			if err != nil {
				return fmt.Errorf("marshal new subscription for chatID=%d: %w", newSub.ChatID, err)
			}

			if err := b.Put(k, newData); err != nil {
				return fmt.Errorf("put new subscription for chatID=%d: %w", newSub.ChatID, err)
			}

			migratedCount++
		}

		// Note: In production, you might want to log migratedCount
		// But since we don't have access to logger here, we just return success
		_ = migratedCount

		return nil
	})
}

// New creates a new instance of MigrationV3
func New() *MigrationV3 {
	return &MigrationV3{}
}
