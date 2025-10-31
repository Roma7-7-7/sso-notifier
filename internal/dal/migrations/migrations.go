package migrations

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"go.etcd.io/bbolt"

	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations/v1"
)

// Migration represents a database migration
type Migration interface {
	// Version returns the migration version number (1, 2, 3, ...)
	Version() int

	// Description returns a human-readable description of what this migration does
	Description() string

	// Up performs the migration on the provided database
	Up(db *bbolt.DB) error
}

var registeredMigrations []Migration

const migrationsBucket = "migrations"

func init() {
	registerMigration(v1.New())
	// Future migrations will be registered here
	// registerMigration(v2.New())
}

func registerMigration(m Migration) {
	registeredMigrations = append(registeredMigrations, m)
}

// RunMigrations executes all pending migrations in order
func RunMigrations(db *bbolt.DB, log *slog.Logger) error {
	log.Info("Starting database migrations")

	// Ensure migrations bucket exists
	if err := ensureMigrationsBucket(db); err != nil {
		return fmt.Errorf("ensure migrations bucket: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("get applied migrations: %w", err)
	}

	// Sort migrations by version
	sort.Slice(registeredMigrations, func(i, j int) bool {
		return registeredMigrations[i].Version() < registeredMigrations[j].Version()
	})

	// Run pending migrations
	appliedCount := 0
	for _, migration := range registeredMigrations {
		version := migration.Version()

		if appliedAt, alreadyApplied := applied[version]; alreadyApplied {
			log.Info("Skipping already-applied migration",
				"version", version,
				"description", migration.Description(),
				"applied_at", appliedAt.Format(time.RFC3339))
			continue
		}

		log.Info("Applying migration",
			"version", version,
			"description", migration.Description())

		start := time.Now()
		if err := migration.Up(db); err != nil {
			log.Error("Migration failed",
				"version", version,
				"description", migration.Description(),
				"error", err)
			return fmt.Errorf("migration v%d failed: %w", version, err)
		}
		duration := time.Since(start)

		if err := recordMigration(db, version); err != nil {
			return fmt.Errorf("record migration v%d: %w", version, err)
		}

		appliedCount++
		log.Info("Migration applied successfully",
			"version", version,
			"duration", duration)
	}

	if appliedCount == 0 {
		log.Info("No pending migrations found")
	} else {
		log.Info("All migrations completed successfully",
			"applied_count", appliedCount)
	}

	return nil
}

func ensureMigrationsBucket(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(migrationsBucket))
		return err
	})
}

func getAppliedMigrations(db *bbolt.DB) (map[int]time.Time, error) {
	applied := make(map[int]time.Time)

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(migrationsBucket))
		if b == nil {
			// Bucket doesn't exist yet, no migrations applied
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var version int
			if _, err := fmt.Sscanf(string(k), "v%d", &version); err != nil {
				return fmt.Errorf("parse version from key %s: %w", k, err)
			}

			timestamp, err := time.Parse(time.RFC3339, string(v))
			if err != nil {
				return fmt.Errorf("parse timestamp for v%d: %w", version, err)
			}

			applied[version] = timestamp
			return nil
		})
	})

	return applied, err
}

func recordMigration(db *bbolt.DB, version int) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(migrationsBucket))
		if b == nil {
			return fmt.Errorf("migrations bucket not found")
		}
		key := []byte(fmt.Sprintf("v%d", version))
		value := []byte(time.Now().Format(time.RFC3339))
		return b.Put(key, value)
	})
}
