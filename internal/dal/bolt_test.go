package dal_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations"
)

type BoltDBTestSuite struct {
	suite.Suite
	db     *bbolt.DB
	store  *dal.BoltDB
	tmpDir string
}

// SetupSuite runs ONCE before all tests in the suite
func (s *BoltDBTestSuite) SetupSuite() {
	// Create temporary directory
	s.tmpDir = s.T().TempDir()

	// Open database
	dbPath := filepath.Join(s.tmpDir, "test.db")
	db, err := bbolt.Open(dbPath, 0600, nil)
	s.Require().NoError(err)

	// Run migrations
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Quiet during tests
	}))
	err = migrations.RunMigrations(db, log)
	s.Require().NoError(err)

	s.db = db
	s.store, err = dal.NewBoltDB(db)
	s.Require().NoError(err)
}

// TearDownSuite runs ONCE after all tests
func (s *BoltDBTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

// TearDownTest runs after EACH test (cleanup data, not DB)
func (s *BoltDBTestSuite) TearDownTest() {
	allBuckets := []string{
		"alerts",
		"notifications",
		"shutdowns",
		"subscriptions",
	}
	// Clean up test data between tests
	// This keeps the same DB but removes test data
	err := s.db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range allBuckets {
			b := tx.Bucket([]byte(bucket))
			s.Require().NotNilf(b, "bucket: %v", bucket)
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				s.Require().NoError(b.Delete(k))
			}
		}
		return nil
	})
	s.Require().NoError(err)
}

// Run the suite
func TestBoltDBTestSuite(t *testing.T) {
	suite.Run(t, new(BoltDBTestSuite))
}
