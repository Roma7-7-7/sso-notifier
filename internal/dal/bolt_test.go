package dal

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"

	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations"
)

type BoltDBTestSuite struct {
	suite.Suite
	db     *bbolt.DB
	store  *BoltDB
	now    *nowWrapper
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
	s.store, err = NewBoltDB(db)
	s.Require().NoError(err)
	s.now = &nowWrapper{}
	s.store.now = func() time.Time {
		return s.now.Call()
	}
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

	s.now.Reset()
	s.store.now = func() time.Time {
		return s.now.Call()
	}
}

// Run the suite
func TestBoltDBTestSuite(t *testing.T) {
	suite.Run(t, new(BoltDBTestSuite))
}

type nowWrapper struct {
	now func() time.Time
}

func (w *nowWrapper) Call() time.Time {
	if w.now != nil {
		return w.now()
	}
	return time.Now()
}

func (w *nowWrapper) SetF(now func() time.Time) {
	w.now = now
}

func (w *nowWrapper) Set(v time.Time) {
	w.now = func() time.Time {
		return v
	}
}

func (w *nowWrapper) Reset() {
	w.now = time.Now
}
