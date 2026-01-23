package dal_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"go.etcd.io/bbolt"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/migrations"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

func (s *BoltDBTestSuite) TestBoltDB_GetShutdowns() {
	today := dal.DateByTime(time.Now())
	tomorrow := dal.TomorrowDateByTime(time.Now().AddDate(0, 0, 1))
	shutdowns, ok, err := s.store.GetShutdowns(today)
	s.Require().NoError(err)
	if s.False(ok) {
		s.Empty(shutdowns)
	}
	shutdowns, ok, err = s.store.GetShutdowns(tomorrow)
	s.Require().NoError(err)
	if s.False(ok) {
		s.Empty(shutdowns)
	}

	s.Require().NoError(s.store.PutShutdowns(today, testutil.NewShutdowns().Build()))
	shutdowns, ok, err = s.store.GetShutdowns(today)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(testutil.NewShutdowns().Build(), shutdowns)
	}
	shutdowns, ok, err = s.store.GetShutdowns(tomorrow)
	s.Require().NoError(err)
	if s.False(ok) {
		s.Empty(shutdowns)
	}

	s.Require().NoError(s.store.PutShutdowns(tomorrow, testutil.NewShutdowns().Build()))
	shutdowns, ok, err = s.store.GetShutdowns(today)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(testutil.NewShutdowns().Build(), shutdowns)
	}
	shutdowns, ok, err = s.store.GetShutdowns(tomorrow)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(testutil.NewShutdowns().Build(), shutdowns)
	}
}

type EmergencyTestSuite struct {
	suite.Suite
	db        *bbolt.DB
	store     *dal.BoltDB
	clockMock *clock.Mock
	tmpDir    string
}

func (s *EmergencyTestSuite) SetupSuite() {
	s.tmpDir = s.T().TempDir()

	dbPath := filepath.Join(s.tmpDir, "test.db")
	db, err := bbolt.Open(dbPath, 0600, nil)
	s.Require().NoError(err)

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	err = migrations.RunMigrations(db, log)
	s.Require().NoError(err)

	s.db = db
	s.clockMock = clock.NewMockF(time.Now)

	s.store, err = dal.NewBoltDB(db, s.clockMock)
	s.Require().NoError(err)
}

func (s *EmergencyTestSuite) TearDownSuite() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *EmergencyTestSuite) TearDownTest() {
	allBuckets := []string{
		"alerts",
		"notifications",
		"shutdowns",
		"subscriptions",
	}
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

	s.clockMock.SetF(time.Now)
}

func TestEmergencyTestSuite(t *testing.T) {
	suite.Run(t, new(EmergencyTestSuite))
}

func (s *EmergencyTestSuite) TestEmergencyState_DefaultInactive() {
	state, err := s.store.GetEmergencyState()
	s.Require().NoError(err)
	s.False(state.Active)
	s.True(state.StartedAt.IsZero())
}

func (s *EmergencyTestSuite) TestEmergencyState_SetAndGet() {
	now := time.Now().UTC().Truncate(time.Second)

	newState := dal.EmergencyState{
		Active:    true,
		StartedAt: now,
	}

	err := s.store.SetEmergencyState(newState)
	s.Require().NoError(err)

	state, err := s.store.GetEmergencyState()
	s.Require().NoError(err)
	s.True(state.Active)
	s.Equal(now, state.StartedAt.UTC().Truncate(time.Second))
}

func (s *EmergencyTestSuite) TestEmergencyState_Clear() {
	now := time.Now().UTC()

	err := s.store.SetEmergencyState(dal.EmergencyState{
		Active:    true,
		StartedAt: now,
	})
	s.Require().NoError(err)

	err = s.store.SetEmergencyState(dal.EmergencyState{Active: false})
	s.Require().NoError(err)

	state, err := s.store.GetEmergencyState()
	s.Require().NoError(err)
	s.False(state.Active)
}
