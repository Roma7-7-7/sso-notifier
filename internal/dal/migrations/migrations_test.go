package migrations

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestRunMigrations_EmptyDatabase(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create logger
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Run migrations
	err = RunMigrations(db, log)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify migrations bucket exists
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("migrations"))
		if b == nil {
			t.Fatal("migrations bucket not created")
		}

		for _, m := range registeredMigrations {
			record := b.Get([]byte(fmt.Sprintf("v%d", m.Version())))
			if record == nil {
				t.Fatalf("migration %d not found in database", m.Version())
			}
			t.Logf("migration %d found in database: %s", m.Version(), string(record))
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to verify migrations: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create logger
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Run migrations first time
	err = RunMigrations(db, log)
	if err != nil {
		t.Fatalf("First RunMigrations failed: %v", err)
	}

	// Run migrations second time (should be idempotent)
	err = RunMigrations(db, log)
	if err != nil {
		t.Fatalf("Second RunMigrations failed: %v", err)
	}

	// Verify migrations bucket still exists and has correct records
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("migrations"))
		if b == nil {
			t.Fatal("migrations bucket not found after second run")
		}

		// Count migration records
		count := 0
		err := b.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
		if err != nil {
			return err
		}

		// We should have all registered migrations
		if count != len(registeredMigrations) {
			t.Fatalf("Expected %d migration records, got %d", len(registeredMigrations), count)
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to verify migrations: %v", err)
	}
}

func TestRunMigrations_CreatesRequiredBuckets(t *testing.T) {
	// Create temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer db.Close()

	// Create logger with minimal output for cleaner test logs
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Run migrations
	err = RunMigrations(db, log)
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify all required buckets were created
	err = db.View(func(tx *bbolt.Tx) error {
		// Check migrations bucket
		if tx.Bucket([]byte("migrations")) == nil {
			t.Fatal("migrations bucket was not created")
		}

		// Check shutdowns bucket (created by v2)
		if tx.Bucket([]byte("shutdowns")) == nil {
			t.Fatal("shutdowns bucket was not created by v2 migration")
		}

		// Check subscriptions bucket (created by v2)
		if tx.Bucket([]byte("subscriptions")) == nil {
			t.Fatal("subscriptions bucket was not created by v2 migration")
		}

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to verify buckets: %v", err)
	}
}
