package dal_test

import (
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/dal/testutil"
)

func (s *BoltDBTestSuite) TestBoltDB_CountSubscriptions() {
	count, err := s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(0, count)

	err = s.store.PutSubscription(testutil.NewSubscription(1).Build())
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(1, count)

	err = s.store.PutSubscription(testutil.NewSubscription(2).Build())
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(2, count)

	err = s.store.PutSubscription(testutil.NewSubscription(1).Build()) // same chat ID
	s.Require().NoError(err, "error putting subscription")
	count, err = s.store.CountSubscriptions()
	s.Require().NoError(err, "error counting subscriptions")
	s.Require().Equal(2, count)
}

func (s *BoltDBTestSuite) TestBoltDB_ExistsSubscription() {
	ok, err := s.store.ExistsSubscription(1)
	s.Require().NoError(err, "error checking subscription")
	s.Require().False(ok)

	err = s.store.PutSubscription(testutil.NewSubscription(1).Build())
	s.Require().NoError(err, "error putting subscription")
	ok, err = s.store.ExistsSubscription(1)
	s.Require().NoError(err, "error checking subscription")
	s.Require().True(ok)
}

func (s *BoltDBTestSuite) TestBoltDB_GetSubscription() {
	startAt := time.Now()
	s.Require().NoError(s.store.PutSubscription(testutil.NewSubscription(1).Build()))

	actual, ok, err := s.store.GetSubscription(1)
	s.Require().NoError(err, "error getting subscription")
	if s.Assert().True(ok) {
		expected := testutil.NewSubscription(1).WithCreatedAt(actual.CreatedAt).Build()
		s.Assert().GreaterOrEqual(expected.CreatedAt, startAt, "subscription's CreatedAt must be after test's start at")
		s.Assert().Equal(expected, actual)
	}

	actual, ok, err = s.store.GetSubscription(2)
	s.Require().NoError(err, "error getting subscription")
	if s.Assert().False(ok) {
		s.Assert().Nil(actual)
	}
}

func (s *BoltDBTestSuite) TestBoltDB_GetAllSubscriptions() {
	s.Require().NoError(s.store.PutSubscription(testutil.NewSubscription(1).Build()))
	s.Require().NoError(s.store.PutSubscription(testutil.NewSubscription(2).Build()))
	s.Require().NoError(s.store.PutSubscription(testutil.NewSubscription(3).Build()))

	actual, err := s.store.GetAllSubscriptions()
	s.Require().NoError(err, "error getting all subscriptions")

	if s.Assert().Len(actual, 3) {
		expected := []dal.Subscription{
			testutil.NewSubscription(1).WithCreatedAt(actual[0].CreatedAt).Build(),
			testutil.NewSubscription(2).WithCreatedAt(actual[1].CreatedAt).Build(),
			testutil.NewSubscription(3).WithCreatedAt(actual[2].CreatedAt).Build(),
		}

		s.Assert().Equal(expected, actual)
	}
}
