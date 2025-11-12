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

func (s *BoltDBTestSuite) TestBoltDB_Purge() {
	// Setup: Create 3 subscriptions with different chatIDs
	chatID1 := int64(101)
	chatID2 := int64(102)
	chatID3 := int64(103)

	subscription1 := NewSubscription(chatID1).WithGroups("1", "2").Build()
	subscription2 := NewSubscription(chatID2).WithGroups("3", "4").Build()
	subscription3 := NewSubscription(chatID3).WithGroups("5", "6").Build()

	s.Require().NoError(s.store.PutSubscription(subscription1))
	s.Require().NoError(s.store.PutSubscription(subscription2))
	s.Require().NoError(s.store.PutSubscription(subscription3))

	// Create dates for notifications
	date1 := Date{Year: 2025, Month: 11, Day: 23}
	date2 := Date{Year: 2025, Month: 11, Day: 24}
	sentAt := time.Now().UTC()

	// Create 2 notification states for each subscription
	notifications := []NotificationState{
		// For chatID1
		NewNotificationState(chatID1, date1).
			WithSentAt(sentAt).
			WithHash("1", "hash1_chatID1_date1").
			WithHash("2", "hash2_chatID1_date1").
			Build(),
		NewNotificationState(chatID1, date2).
			WithSentAt(sentAt.Add(1*time.Hour)).
			WithHash("1", "hash1_chatID1_date2").
			WithHash("2", "hash2_chatID1_date2").
			Build(),
		// For chatID2
		NewNotificationState(chatID2, date1).
			WithSentAt(sentAt.Add(2*time.Hour)).
			WithHash("3", "hash1_chatID2_date1").
			WithHash("4", "hash2_chatID2_date1").
			Build(),
		NewNotificationState(chatID2, date2).
			WithSentAt(sentAt.Add(3*time.Hour)).
			WithHash("3", "hash1_chatID2_date2").
			WithHash("4", "hash2_chatID2_date2").
			Build(),
		// For chatID3
		NewNotificationState(chatID3, date1).
			WithSentAt(sentAt.Add(4*time.Hour)).
			WithHash("5", "hash1_chatID3_date1").
			WithHash("6", "hash2_chatID3_date1").
			Build(),
		NewNotificationState(chatID3, date2).
			WithSentAt(sentAt.Add(5*time.Hour)).
			WithHash("5", "hash1_chatID3_date2").
			WithHash("6", "hash2_chatID3_date2").
			Build(),
	}

	for i, notif := range notifications {
		s.Require().NoErrorf(s.store.PutNotificationState(notif), "PutNotificationState err for notification %d", i)
	}

	// Create 2 alerts for each subscription
	alerts := []struct {
		key    AlertKey
		sentAt time.Time
	}{
		// For chatID1
		{BuildAlertKey(chatID1, "2025-11-23", "10:00", string(ON), "1"), sentAt},
		{BuildAlertKey(chatID1, "2025-11-23", "14:00", string(MAYBE), "2"), sentAt.Add(1 * time.Hour)},
		// For chatID2
		{BuildAlertKey(chatID2, "2025-11-24", "10:00", string(ON), "3"), sentAt.Add(2 * time.Hour)},
		{BuildAlertKey(chatID2, "2025-11-24", "14:00", string(OFF), "4"), sentAt.Add(3 * time.Hour)},
		// For chatID3
		{BuildAlertKey(chatID3, "2025-11-25", "10:00", string(ON), "5"), sentAt.Add(4 * time.Hour)},
		{BuildAlertKey(chatID3, "2025-11-25", "14:00", string(MAYBE), "6"), sentAt.Add(5 * time.Hour)},
	}

	for i, alert := range alerts {
		s.Require().NoErrorf(s.store.PutAlert(alert.key, alert.sentAt), "PutAlert err for alert %d", i)
	}

	// BEFORE PURGE: Verify all subscriptions exist
	sub1, ok1, err1 := s.store.GetSubscription(chatID1)
	s.Require().NoError(err1)
	s.Require().True(ok1, "subscription for chatID1 should exist")
	s.Equal(subscription1.ChatID, sub1.ChatID)

	sub2, ok2, err2 := s.store.GetSubscription(chatID2)
	s.Require().NoError(err2)
	s.Require().True(ok2, "subscription for chatID2 should exist")
	s.Equal(subscription2.ChatID, sub2.ChatID)

	sub3, ok3, err3 := s.store.GetSubscription(chatID3)
	s.Require().NoError(err3)
	s.Require().True(ok3, "subscription for chatID3 should exist")
	s.Equal(subscription3.ChatID, sub3.ChatID)

	// BEFORE PURGE: Verify all notifications exist
	for i, notif := range notifications {
		dateMap := map[string]Date{
			date1.ToKey(): date1,
			date2.ToKey(): date2,
		}
		date := dateMap[notif.Date]
		state, ok, err := s.store.GetNotificationState(notif.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for notification %d", i)
		s.Truef(ok, "NotificationState should exist for notification %d (chatID=%d, date=%s)", i, notif.ChatID, notif.Date)
		s.Equalf(notif.ChatID, state.ChatID, "Invalid ChatID for notification %d", i)
	}

	// BEFORE PURGE: Verify all alerts exist
	for i, alert := range alerts {
		storedSentAt, ok, err := s.store.GetAlert(alert.key)
		s.Require().NoErrorf(err, "GetAlert err for alert %d", i)
		s.Truef(ok, "Alert should exist for alert %d (key=%s)", i, alert.key)
		s.Equalf(alert.sentAt.Truncate(time.Second), storedSentAt.UTC().Truncate(time.Second), "Invalid sentAt for alert %d", i)
	}

	// PURGE: Delete all data for chatID2
	err := s.store.Purge(chatID2)
	s.Require().NoError(err, "Purge should not return an error")

	// AFTER PURGE: Verify subscription for chatID2 is deleted
	_, ok, err := s.store.GetSubscription(chatID2)
	s.Require().NoError(err)
	s.False(ok, "subscription for chatID2 should be deleted after Purge")

	// AFTER PURGE: Verify subscriptions for chatID1 and chatID3 still exist
	sub1After, ok1After, err1After := s.store.GetSubscription(chatID1)
	s.Require().NoError(err1After)
	s.Require().True(ok1After, "subscription for chatID1 should still exist after Purge")
	s.Equal(subscription1.ChatID, sub1After.ChatID)

	sub3After, ok3After, err3After := s.store.GetSubscription(chatID3)
	s.Require().NoError(err3After)
	s.Require().True(ok3After, "subscription for chatID3 should still exist after Purge")
	s.Equal(subscription3.ChatID, sub3After.ChatID)

	// AFTER PURGE: Verify notifications for chatID2 are deleted, others still exist
	for i, notif := range notifications {
		dateMap := map[string]Date{
			date1.ToKey(): date1,
			date2.ToKey(): date2,
		}
		date := dateMap[notif.Date]
		state, ok, err := s.store.GetNotificationState(notif.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for notification %d", i)

		if notif.ChatID == chatID2 {
			s.Falsef(ok, "NotificationState for chatID2 should be deleted (notification %d)", i)
			s.Empty(state.Hashes, "NotificationState for chatID2 should be empty (notification %d)", i)
		} else {
			s.Truef(ok, "NotificationState for chatID %d should still exist (notification %d)", notif.ChatID, i)
			s.Equalf(notif.ChatID, state.ChatID, "Invalid ChatID for notification %d", i)
		}
	}

	// AFTER PURGE: Verify alerts for chatID2 are deleted, others still exist
	for i, alert := range alerts {
		storedSentAt, ok, err := s.store.GetAlert(alert.key)
		s.Require().NoErrorf(err, "GetAlert err for alert %d", i)

		// Extract chatID from the alert key to determine expected behavior
		var alertChatID int64
		switch {
		case i < 2:
			alertChatID = chatID1
		case i < 4:
			alertChatID = chatID2
		default:
			alertChatID = chatID3
		}

		if alertChatID == chatID2 {
			s.Falsef(ok, "Alert for chatID2 should be deleted (alert %d, key=%s)", i, alert.key)
			s.Equal(time.Time{}, storedSentAt, "SentAt should be empty for deleted alert %d", i)
		} else {
			s.Truef(ok, "Alert for chatID %d should still exist (alert %d, key=%s)", alertChatID, i, alert.key)
			s.Equalf(alert.sentAt.Truncate(time.Second), storedSentAt.UTC().Truncate(time.Second), "Invalid sentAt for alert %d", i)
		}
	}

	// Verify counts after purge
	totalSubscriptions, err := s.store.CountSubscriptions()
	s.Require().NoError(err)
	s.Equal(2, totalSubscriptions, "Should have 2 subscriptions left after purging chatID2")

	// Test purging non-existent chatID (should not error)
	err = s.store.Purge(999)
	s.Require().NoError(err, "Purging non-existent chatID should not return an error")

	// Verify nothing else was affected
	totalSubscriptionsAfterNoop, err := s.store.CountSubscriptions()
	s.Require().NoError(err)
	s.Equal(2, totalSubscriptionsAfterNoop, "Should still have 2 subscriptions after purging non-existent chatID")
}
