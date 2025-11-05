package v5

import (
	"go.etcd.io/bbolt"
)

// MigrationV5 creates the alerts bucket for tracking 10-minute advance alerts
type MigrationV5 struct{}

// Version returns the migration version
func (m *MigrationV5) Version() int {
	return 5 //nolint:mnd // version 5
}

// Description returns a human-readable description of the migration
func (m *MigrationV5) Description() string {
	return "Create alerts bucket for tracking 10-minute advance alerts"
}

// Up performs the migration
func (m *MigrationV5) Up(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("alerts"))
		return err
	})
}

// New creates a new instance of MigrationV5
func New() *MigrationV5 {
	return &MigrationV5{}
}
