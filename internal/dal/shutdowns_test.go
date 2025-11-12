package dal_test

import (
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
)

func (s *BoltDBTestSuite) TestBoltDB_GetShutdowns() {
	today := dal.TodayDate(time.UTC)
	tomorrow := dal.TomorrowDate(time.UTC)
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
