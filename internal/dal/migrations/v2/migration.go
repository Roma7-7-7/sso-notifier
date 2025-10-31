package v2

import (
	"go.etcd.io/bbolt"
)

// MigrationV2 creates the application buckets for shutdowns and subscriptions
type MigrationV2 struct{}

// Version returns the migration version
func (m *MigrationV2) Version() int {
	return 2 //nolint:mnd // version 2
}

// Description returns a human-readable description of the migration
func (m *MigrationV2) Description() string {
	return "Create shutdowns and subscriptions buckets"
}

// Up performs the migration
func (m *MigrationV2) Up(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		// Create shutdowns bucket if it doesn't exist
		if _, err := tx.CreateBucketIfNotExists([]byte("shutdowns")); err != nil {
			return err
		}

		// Create subscriptions bucket if it doesn't exist
		if _, err := tx.CreateBucketIfNotExists([]byte("subscriptions")); err != nil {
			return err
		}

		return nil
	})
}

// New creates a new instance of MigrationV2
func New() *MigrationV2 {
	return &MigrationV2{}
}
