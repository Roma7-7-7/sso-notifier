package dal

import (
	"time"
)

func (s *BoltDBTestSuite) TestBoltDB_Get_Put_Delete_Alert() {
	key1 := BuildAlertKey(1, "12025-11-23", "11:00", string(ON), "1")
	key2 := BuildAlertKey(2, "12025-11-23", "11:00", string(ON), "1")
	key3 := BuildAlertKey(1, "12025-11-24", "11:00", string(ON), "1")
	key4 := BuildAlertKey(1, "12025-11-23", "11:30", string(ON), "1")
	key5 := BuildAlertKey(1, "12025-11-23", "11:00", string(MAYBE), "1")
	key6 := BuildAlertKey(1, "12025-11-23", "11:00", string(ON), "2")
	unexistingKey := BuildAlertKey(7, "12025-11-23", "11:00", string(ON), "7")

	allKeys := []AlertKey{key1, key2, key3, key4, key5, key6}

	for _, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Falsef(ok, "Alert should not be present for key: %s", key) {
			s.Emptyf(alert, "Alert should not be present for key: %s", key)
		}
	}

	sentAt := time.Now()
	for i, key := range allKeys {
		s.Require().NoErrorf(s.store.PutAlert(key, sentAt.Add(time.Duration(i)*time.Hour)), "PutAlert err for key: %s", key)
	}

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Truef(ok, "Alert should be present for key: %s", key) {
			s.Equalf(sentAt.Add(time.Duration(i)*time.Hour).Truncate(time.Second), alert, "Invalid alert for key: %s", key)
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
			s.Equalf(expected, alert, "Invalid alert for key: %s", key)
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
	key1 := BuildAlertKey(1, "2025-11-23", "11:00", string(ON), "1")
	key2 := BuildAlertKey(1, "2025-11-23", "11:30", string(MAYBE), "1")
	key3 := BuildAlertKey(2, "2025-11-24", "11:00", string(ON), "2")
	key4 := BuildAlertKey(2, "2025-11-25", "11:30", string(OFF), "2")
	key5 := BuildAlertKey(3, "2025-11-25", "11:30", string(OFF), "3")
	key6 := BuildAlertKey(3, "2025-11-25", "11:30", string(MAYBE), "3")

	sentAt := time.Now()
	allKeys := []AlertKey{key1, key2, key3, key4, key5, key6}
	for i, key := range allKeys {
		s.Require().NoErrorf(s.store.PutAlert(key, sentAt.Add(time.Duration(i)*time.Hour)), "PutAlert err for key: %s", key)
	}

	for i, key := range allKeys {
		alert, ok, err := s.store.GetAlert(key)
		s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
		if s.Truef(ok, "Alert should be present for key: %s", key) {
			expected := sentAt.Add(time.Duration(i) * time.Hour).Truncate(time.Second)
			s.Equalf(expected, alert, "Invalid alert for key: %s", key)
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
			s.Equal(expectedAlert, alert, "Invalid alert for key: %s", key)
		}
	}
}

func (s *BoltDBTestSuite) mustGetAlert(key AlertKey) time.Time {
	alert, ok, err := s.store.GetAlert(key)
	s.Require().NoErrorf(err, "GetAlert err for key: %s", key)
	s.Require().Truef(ok, "Alert should be present for key: %s", key)
	return alert
}
