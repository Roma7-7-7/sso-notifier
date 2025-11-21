package dal_test

import (
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
)

func (s *BoltDBTestSuite) TestBoltDB_Get_Put_Delete_Alert() {
	key1 := dal.BuildAlertKey(1, "12025-11-23", "11:00", string(dal.ON), "1")
	key2 := dal.BuildAlertKey(2, "12025-11-23", "11:00", string(dal.ON), "1")
	key3 := dal.BuildAlertKey(1, "12025-11-24", "11:00", string(dal.ON), "1")
	key4 := dal.BuildAlertKey(1, "12025-11-23", "11:30", string(dal.ON), "1")
	key5 := dal.BuildAlertKey(1, "12025-11-23", "11:00", string(dal.MAYBE), "1")
	key6 := dal.BuildAlertKey(1, "12025-11-23", "11:00", string(dal.ON), "2")
	unexistingKey := dal.BuildAlertKey(7, "12025-11-23", "11:00", string(dal.ON), "7")

	allKeys := []dal.AlertKey{key1, key2, key3, key4, key5, key6}

	for _, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Falsef(ok, "Alert should not be present for key: %s", key) {
			s.Emptyf(alert, "Alert should not be present for key: %s", key)
		}
	}

	sentAt := time.Now().UTC()
	for i, key := range allKeys {
		s.Require().NoErrorf(s.store.PutAlert(key, sentAt.Add(time.Duration(i)*time.Hour)), "PutAlert err for key: %s", key)
	}

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Truef(ok, "Alert should be present for key: %s", key) {
			s.Equalf(sentAt.Add(time.Duration(i)*time.Hour).Truncate(time.Second), alert.UTC(), "Invalid alert for key: %s", key)
		}
	}

	// change sent at for key3
	s.Require().NoError(s.store.PutAlert(key3, sentAt.Add(7*time.Hour)))

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Truef(ok, "Alert should be present for key: %s", key) {
			expected := sentAt.Add(time.Duration(i) * time.Hour).Truncate(time.Second)
			if key3 == key {
				expected = sentAt.Add(7 * time.Hour).Truncate(time.Second)
			}
			s.Equalf(expected, alert.UTC(), "Invalid alert for key: %s", key)
		}
	}

	// delete for key4
	s.Require().NoErrorf(s.store.DeleteAlert(key4), "DeleteAlert err for key: %s", key4)

	for _, key := range allKeys {
		_, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		expected := true
		if key == key4 {
			expected = false
		}

		s.Equal(expected, ok, "Invalid ok for key: %s", key)
	}

	// delete non existing alert
	s.NoErrorf(s.store.DeleteAlert(unexistingKey), "DeleteAlert err for key: %s", key5)
}

func (s *BoltDBTestSuite) TestBoltDB_DeleteAlerts() {
	key1 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "1")
	key2 := dal.BuildAlertKey(1, "2025-11-23", "11:30", string(dal.MAYBE), "1")
	key3 := dal.BuildAlertKey(2, "2025-11-24", "11:00", string(dal.ON), "2")
	key4 := dal.BuildAlertKey(2, "2025-11-25", "11:30", string(dal.OFF), "2")
	key5 := dal.BuildAlertKey(3, "2025-11-25", "11:30", string(dal.OFF), "3")
	key6 := dal.BuildAlertKey(3, "2025-11-25", "11:30", string(dal.MAYBE), "3")

	sentAt := time.Now().UTC()
	allKeys := []dal.AlertKey{key1, key2, key3, key4, key5, key6}
	for i, key := range allKeys {
		s.Require().NoErrorf(s.store.PutAlert(key, sentAt.Add(time.Duration(i)*time.Hour)), "PutAlert err for key: %s", key)
	}

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Truef(ok, "Alert should be present for key: %s", key) {
			expected := sentAt.Add(time.Duration(i) * time.Hour).Truncate(time.Second)
			s.Equalf(expected, alert.UTC(), "Invalid alert for key: %s", key)
		}
	}

	s.Require().NoError(s.store.DeleteAlerts(2))

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		expectedAlert := sentAt.Add(time.Duration(i) * time.Hour).Truncate(time.Second)
		expectedOk := true
		if key == key3 || key == key4 {
			expectedOk = false
			expectedAlert = time.Time{}
		}
		if s.Equal(expectedOk, ok, "Invalid ok for key: %s", key) {
			s.Equal(expectedAlert, alert.UTC(), "Invalid alert for key: %s", key)
		}
	}
}

func (s *BoltDBTestSuite) TestBoltDB_CleanupAlerts() {
	key1 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "1")
	key2 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "2")
	key3 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "3")
	key4 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "4")
	key5 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "5")
	key6 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "6")
	key7 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "7")
	key8 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "8")
	key9 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "9")
	key10 := dal.BuildAlertKey(1, "2025-11-23", "11:00", string(dal.ON), "10")

	s.Require().NoError(s.store.PutAlert(key1, time.Date(2025, time.November, 23, 1, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key1)
	s.Require().NoError(s.store.PutAlert(key2, time.Date(2025, time.November, 23, 2, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key2)
	s.Require().NoError(s.store.PutAlert(key3, time.Date(2025, time.November, 23, 3, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key3)
	s.Require().NoError(s.store.PutAlert(key4, time.Date(2025, time.November, 23, 4, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key4)
	s.Require().NoError(s.store.PutAlert(key5, time.Date(2025, time.November, 23, 5, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key5)
	s.Require().NoError(s.store.PutAlert(key6, time.Date(2025, time.November, 23, 6, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key6)
	s.Require().NoError(s.store.PutAlert(key7, time.Date(2025, time.November, 23, 7, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key7)
	s.Require().NoError(s.store.PutAlert(key8, time.Date(2025, time.November, 23, 8, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key8)
	s.Require().NoError(s.store.PutAlert(key9, time.Date(2025, time.November, 23, 9, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key9)
	s.Require().NoError(s.store.PutAlert(key10, time.Date(2025, time.November, 23, 10, 0, 0, 0, time.UTC)), "PutAlert err for key: %s", key10)

	s.clockMock.Set(time.Date(2025, time.November, 23, 11, 0, 0, 0, time.UTC))

	count, err := s.store.CountAlerts()
	s.Require().NoError(err)
	s.Require().Equal(10, count)

	s.Require().NoError(s.store.CleanupAlerts(12 * time.Hour))
	count, err = s.store.CountAlerts()
	s.Require().NoError(err)
	s.Require().Equal(10, count)

	s.Require().NoError(s.store.CleanupAlerts(6 * time.Hour))
	count, err = s.store.CountAlerts()
	s.Require().NoError(err)
	s.Require().Equal(5, count)
	alert, ok, err := s.store.GetAlert(key6)
	s.Require().NoError(err)
	if s.Assert().True(ok) {
		s.Assert().Equal(time.Date(2025, time.November, 23, 6, 0, 0, 0, time.UTC), alert)
	}

	s.Require().NoError(s.store.CleanupAlerts(3 * time.Hour))
	count, err = s.store.CountAlerts()
	s.Require().NoError(err)
	s.Require().Equal(2, count)
	alert, ok, err = s.store.GetAlert(key9)
	s.Require().NoError(err)
	if s.Assert().True(ok) {
		s.Assert().Equal(time.Date(2025, time.November, 23, 9, 0, 0, 0, time.UTC), alert)
	}

	s.Require().NoError(s.store.CleanupAlerts(time.Hour + time.Minute))
	count, err = s.store.CountAlerts()
	s.Require().NoError(err)
	s.Require().Equal(1, count)
	alert, ok, err = s.store.GetAlert(key10)
	s.Require().NoError(err)
	if s.Assert().True(ok) {
		s.Assert().Equal(time.Date(2025, time.November, 23, 10, 0, 0, 0, time.UTC), alert)
	}
}
