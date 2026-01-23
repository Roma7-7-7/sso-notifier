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
	"github.com/Roma7-7-7/sso-notifier/pkg/clock"
)

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

func (s *EmergencyTestSuite) TestEmergencyNotification_DefaultNotNotified() {
	_, notified, err := s.store.GetEmergencyNotification(123)
	s.Require().NoError(err)
	s.False(notified)
}

func (s *EmergencyTestSuite) TestEmergencyNotification_SetAndGet() {
	chatID := int64(123)
	now := time.Now().UTC().Truncate(time.Second)

	err := s.store.SetEmergencyNotification(chatID, now)
	s.Require().NoError(err)

	sentAt, notified, err := s.store.GetEmergencyNotification(chatID)
	s.Require().NoError(err)
	s.True(notified)
	s.Equal(now, sentAt.UTC().Truncate(time.Second))
}

func (s *EmergencyTestSuite) TestEmergencyNotification_MultipleUsers() {
	chatID1 := int64(101)
	chatID2 := int64(102)
	chatID3 := int64(103)
	now := time.Now().UTC().Truncate(time.Second)

	err := s.store.SetEmergencyNotification(chatID1, now)
	s.Require().NoError(err)

	err = s.store.SetEmergencyNotification(chatID2, now.Add(time.Hour))
	s.Require().NoError(err)

	sentAt1, notified1, err := s.store.GetEmergencyNotification(chatID1)
	s.Require().NoError(err)
	s.True(notified1)
	s.Equal(now, sentAt1.UTC().Truncate(time.Second))

	sentAt2, notified2, err := s.store.GetEmergencyNotification(chatID2)
	s.Require().NoError(err)
	s.True(notified2)
	s.Equal(now.Add(time.Hour), sentAt2.UTC().Truncate(time.Second))

	_, notified3, err := s.store.GetEmergencyNotification(chatID3)
	s.Require().NoError(err)
	s.False(notified3)
}

func (s *EmergencyTestSuite) TestClearAllEmergencyNotifications() {
	chatID1 := int64(101)
	chatID2 := int64(102)
	now := time.Now().UTC()

	err := s.store.SetEmergencyNotification(chatID1, now)
	s.Require().NoError(err)
	err = s.store.SetEmergencyNotification(chatID2, now)
	s.Require().NoError(err)

	_, notified1, err := s.store.GetEmergencyNotification(chatID1)
	s.Require().NoError(err)
	s.True(notified1)

	_, notified2, err := s.store.GetEmergencyNotification(chatID2)
	s.Require().NoError(err)
	s.True(notified2)

	err = s.store.ClearAllEmergencyNotifications()
	s.Require().NoError(err)

	_, notified1After, err := s.store.GetEmergencyNotification(chatID1)
	s.Require().NoError(err)
	s.False(notified1After)

	_, notified2After, err := s.store.GetEmergencyNotification(chatID2)
	s.Require().NoError(err)
	s.False(notified2After)
}

func (s *EmergencyTestSuite) TestPurge_DeletesEmergencyNotification() {
	chatID := int64(123)
	now := time.Now().UTC()

	err := s.store.SetEmergencyNotification(chatID, now)
	s.Require().NoError(err)

	_, notified, err := s.store.GetEmergencyNotification(chatID)
	s.Require().NoError(err)
	s.True(notified)

	sub := dal.Subscription{
		ChatID:    chatID,
		CreatedAt: now,
		Groups:    map[string]struct{}{"1": {}},
	}
	err = s.store.PutSubscription(sub)
	s.Require().NoError(err)

	err = s.store.Purge(chatID)
	s.Require().NoError(err)

	_, notifiedAfter, err := s.store.GetEmergencyNotification(chatID)
	s.Require().NoError(err)
	s.False(notifiedAfter)
}
