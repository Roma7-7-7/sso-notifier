package v1

import (
	"go.etcd.io/bbolt"
)

// MigrationV1 is the bootstrap migration that creates the migrations bucket
type MigrationV1 struct{}

// Version returns the migration version
func (m *MigrationV1) Version() int {
	return 1
}

// Description returns a human-readable description of the migration
func (m *MigrationV1) Description() string {
	return "Bootstrap migration system - create migrations bucket"
}

// Up performs the migration
func (m *MigrationV1) Up(db *bbolt.DB) error {
	// The migrations bucket is created by the migration runner's ensureMigrationsBucket
	// This migration exists primarily to mark that the migration system is initialized
	// We verify that the bucket exists
	return db.View(func(tx *bbolt.Tx) error {
		// Bucket should already exist from ensureMigrationsBucket call
		if tx.Bucket([]byte("migrations")) == nil {
			// This shouldn't happen, but if it does, it's a critical error
			panic("migrations bucket not found during v1 migration")
		}
		return nil
	})
}

// New creates a new instance of MigrationV1
func New() *MigrationV1 {
	return &MigrationV1{}
}
