package dal_test

import (
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
)

func (s *BoltDBTestSuite) TestBoltDB_Get_Put_Delete_NotificationState() {
	date1 := dal.Date{Year: 2025, Month: 11, Day: 23}
	date2 := dal.Date{Year: 2025, Month: 11, Day: 24}
	date3 := dal.Date{Year: 2025, Month: 11, Day: 25}

	chatID1 := int64(1)
	chatID2 := int64(2)

	// Test keys: different combinations of chatID and date
	keys := []struct {
		chatID int64
		date   dal.Date
	}{
		{chatID1, date1},
		{chatID1, date2},
		{chatID1, date3},
		{chatID2, date1},
		{chatID2, date2},
	}

	// Verify all keys don't exist initially
	for _, k := range keys {
		state, ok, err := s.store.GetNotificationState(k.chatID, k.date)
		s.Require().NoErrorf(err, "GetNotificationState err for chatID=%d date=%s", k.chatID, k.date.ToKey())
		if s.Falsef(ok, "NotificationState should not be present for chatID=%d date=%s", k.chatID, k.date.ToKey()) {
			s.Emptyf(state, "NotificationState should not be present for chatID=%d date=%s", k.chatID, k.date.ToKey())
		}
	}

	sentAt := time.Now().UTC()
	states := make([]dal.NotificationState, len(keys))

	// Create and put states
	for i, k := range keys {
		states[i] = testutil.NewNotificationState(k.chatID, k.date).
			WithSentAt(sentAt.Add(time.Duration(i)*time.Hour)).
			WithHash("1", "hash1_"+k.date.ToKey()).
			WithHash("2", "hash2_"+k.date.ToKey()).
			Build()
		s.Require().NoErrorf(s.store.PutNotificationState(states[i]), "PutNotificationState err for chatID=%d date=%s", k.chatID, k.date.ToKey())
	}

	// Verify all states were stored correctly
	for i, k := range keys {
		state, ok, err := s.store.GetNotificationState(k.chatID, k.date)
		s.Require().NoErrorf(err, "GetNotificationState err for chatID=%d date=%s", k.chatID, k.date.ToKey())
		if s.Truef(ok, "NotificationState should be present for chatID=%d date=%s", k.chatID, k.date.ToKey()) {
			s.Equalf(states[i].ChatID, state.ChatID, "Invalid ChatID for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].Date, state.Date, "Invalid Date for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].SentAt.Truncate(time.Second), state.SentAt.UTC().Truncate(time.Second), "Invalid SentAt for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].Hashes, state.Hashes, "Invalid Hashes for chatID=%d date=%s", k.chatID, k.date.ToKey())
		}
	}

	// Update one state (key index 1: chatID1, date2)
	states[1] = testutil.NewNotificationState(keys[1].chatID, keys[1].date).
		WithSentAt(sentAt.Add(10*time.Hour)).
		WithHash("1", "updated_hash1").
		WithHash("2", "updated_hash2").
		WithHash("3", "new_hash3").
		Build()
	s.Require().NoError(s.store.PutNotificationState(states[1]))

	// Verify update was successful
	for i, k := range keys {
		state, ok, err := s.store.GetNotificationState(k.chatID, k.date)
		s.Require().NoErrorf(err, "GetNotificationState err for chatID=%d date=%s", k.chatID, k.date.ToKey())
		if s.Truef(ok, "NotificationState should be present for chatID=%d date=%s", k.chatID, k.date.ToKey()) {
			s.Equalf(states[i].ChatID, state.ChatID, "Invalid ChatID for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].Date, state.Date, "Invalid Date for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].SentAt.Truncate(time.Second), state.SentAt.UTC().Truncate(time.Second), "Invalid SentAt for chatID=%d date=%s", k.chatID, k.date.ToKey())
			s.Equalf(states[i].Hashes, state.Hashes, "Invalid Hashes for chatID=%d date=%s", k.chatID, k.date.ToKey())
		}
	}
}

func (s *BoltDBTestSuite) TestBoltDB_DeleteNotificationStates() {
	date1 := dal.Date{Year: 2025, Month: 11, Day: 23}
	date2 := dal.Date{Year: 2025, Month: 11, Day: 24}
	date3 := dal.Date{Year: 2025, Month: 11, Day: 25}

	chatID1 := int64(1)
	chatID2 := int64(2)
	chatID3 := int64(3)

	sentAt := time.Now().UTC()

	// Create states for multiple users and dates
	states := []dal.NotificationState{
		testutil.NewNotificationState(chatID1, date1).WithSentAt(sentAt).WithHash("1", "hash1").Build(),
		testutil.NewNotificationState(chatID1, date2).WithSentAt(sentAt.Add(1*time.Hour)).WithHash("1", "hash2").Build(),
		testutil.NewNotificationState(chatID2, date1).WithSentAt(sentAt.Add(2*time.Hour)).WithHash("2", "hash3").Build(),
		testutil.NewNotificationState(chatID2, date2).WithSentAt(sentAt.Add(3*time.Hour)).WithHash("2", "hash4").Build(),
		testutil.NewNotificationState(chatID3, date1).WithSentAt(sentAt.Add(4*time.Hour)).WithHash("3", "hash5").Build(),
		testutil.NewNotificationState(chatID3, date3).WithSentAt(sentAt.Add(5*time.Hour)).WithHash("3", "hash6").Build(),
	}

	// Create a map for easy date lookup
	dateMap := map[string]dal.Date{
		date1.ToKey(): date1,
		date2.ToKey(): date2,
		date3.ToKey(): date3,
	}

	// Put all states
	for i, state := range states {
		s.Require().NoErrorf(s.store.PutNotificationState(state), "PutNotificationState err for state %d", i)
	}

	// Verify all states exist
	for i, state := range states {
		date := dateMap[state.Date]
		retrievedState, ok, err := s.store.GetNotificationState(state.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for state %d", i)
		if s.Truef(ok, "NotificationState should be present for state %d", i) {
			s.Equalf(state.SentAt.Truncate(time.Second), retrievedState.SentAt.UTC().Truncate(time.Second), "Invalid SentAt for state %d", i)
		}
	}

	// Delete all states for chatID2
	s.Require().NoError(s.store.DeleteNotificationStates(chatID2))

	// Verify chatID2 states are deleted, others remain
	for i, state := range states {
		date := dateMap[state.Date]
		retrievedState, ok, err := s.store.GetNotificationState(state.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for state %d", i)

		if state.ChatID == chatID2 {
			s.Falsef(ok, "NotificationState should not be present for state %d (chatID2)", i)
			s.Empty(retrievedState.Hashes, "NotificationState should be empty for state %d (chatID2)", i)
		} else {
			s.Truef(ok, "NotificationState should still be present for state %d (chatID %d)", i, state.ChatID)
			s.Equalf(state.SentAt.Truncate(time.Second), retrievedState.SentAt.UTC().Truncate(time.Second), "Invalid SentAt for state %d", i)
		}
	}

	// Delete all states for chatID1
	s.Require().NoError(s.store.DeleteNotificationStates(chatID1))

	// Verify only chatID3 states remain
	for i, state := range states {
		date := dateMap[state.Date]
		_, ok, err := s.store.GetNotificationState(state.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for state %d", i)

		if state.ChatID == chatID3 {
			s.Truef(ok, "NotificationState should still be present for state %d (chatID3)", i)
		} else {
			s.Falsef(ok, "NotificationState should not be present for state %d (chatID %d)", i, state.ChatID)
		}
	}

	// Delete non-existing chatID (should not error)
	s.NoError(s.store.DeleteNotificationStates(999))

	// Verify only chatID3 states remain
	for i, state := range states {
		date := dateMap[state.Date]
		_, ok, err := s.store.GetNotificationState(state.ChatID, date)
		s.Require().NoErrorf(err, "GetNotificationState err for state %d", i)

		if state.ChatID == chatID3 {
			s.Truef(ok, "NotificationState should still be present for state %d (chatID3)", i)
		} else {
			s.Falsef(ok, "NotificationState should not be present for state %d (chatID %d)", i, state.ChatID)
		}
	}
}
