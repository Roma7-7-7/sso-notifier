package dal

import (
	"fmt"
	"strconv"
	"time"
)

func (s *BoltDBTestSuite) TestBoltDB_GetShutdowns() {
	today := TodayDate(time.UTC)
	tomorrow := TomorrowDate(time.UTC)
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

	s.Require().NoError(s.store.PutShutdowns(today, NewShutdowns().Build()))
	shutdowns, ok, err = s.store.GetShutdowns(today)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(NewShutdowns().Build(), shutdowns)
	}
	shutdowns, ok, err = s.store.GetShutdowns(tomorrow)
	s.Require().NoError(err)
	if s.False(ok) {
		s.Empty(shutdowns)
	}

	s.Require().NoError(s.store.PutShutdowns(tomorrow, NewShutdowns().Build()))
	shutdowns, ok, err = s.store.GetShutdowns(today)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(NewShutdowns().Build(), shutdowns)
	}
	shutdowns, ok, err = s.store.GetShutdowns(tomorrow)
	s.Require().NoError(err)
	if s.True(ok) {
		s.Equal(NewShutdowns().Build(), shutdowns)
	}
}

// ShutdownsBuilder provides fluent API for building test shutdowns
type ShutdownsBuilder struct {
	shutdowns Shutdowns
}

// NewShutdowns creates a new shutdowns builder with defaults
func NewShutdowns() *ShutdownsBuilder {
	return &ShutdownsBuilder{
		shutdowns: Shutdowns{
			Date: "2025-11-23",
			Periods: []Period{
				{"00:00", "00:30"},
				{"00:30", "01:00"},
				{"01:00", "01:30"},
				{"01:30", "02:00"},
				{"02:00", "02:30"},
				{"02:30", "03:00"},
				{"03:00", "03:30"},
				{"03:30", "04:00"},
				{"04:00", "04:30"},
				{"04:30", "05:00"},
				{"05:00", "05:30"},
				{"05:30", "06:00"},
				{"06:00", "06:30"},
				{"06:30", "07:00"},
				{"07:00", "07:30"},
				{"07:30", "08:00"},
				{"08:00", "08:30"},
				{"08:30", "09:00"},
				{"09:00", "09:30"},
				{"09:30", "10:00"},
				{"10:00", "10:30"},
				{"10:30", "11:00"},
				{"11:00", "11:30"},
				{"11:30", "12:00"},
				{"12:00", "12:30"},
				{"12:30", "13:00"},
				{"13:00", "13:30"},
				{"13:30", "14:00"},
				{"14:00", "14:30"},
				{"14:30", "15:00"},
				{"15:00", "15:30"},
				{"15:30", "16:00"},
				{"16:00", "16:30"},
				{"16:30", "17:00"},
				{"17:00", "17:30"},
				{"17:30", "18:00"},
				{"18:00", "18:30"},
				{"18:30", "19:00"},
				{"19:00", "19:30"},
				{"19:30", "20:00"},
				{"20:00", "20:30"},
				{"20:30", "21:00"},
				{"21:00", "21:30"},
				{"21:30", "22:00"},
				{"22:00", "22:30"},
				{"22:30", "23:00"},
				{"23:00", "23:30"},
				{"23:30", "24:00"},
			},
			Groups: map[string]ShutdownGroup{
				"1":  {1, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"2":  {2, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"3":  {3, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"4":  {4, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
				"5":  {5, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"6":  {6, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"7":  {7, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"8":  {8, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
				"9":  {9, parseGroupStatuses("YYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYY")},
				"10": {10, parseGroupStatuses("MYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYY")},
				"11": {11, parseGroupStatuses("NNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNN")},
				"12": {12, parseGroupStatuses("MNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNNNMYYYYYYMNNNNN")},
			},
		},
	}
}

func parseGroupStatuses(s string) []Status {
	if len(s) != 48 {
		panic(fmt.Sprintf("expecting 48 groups, got %d", len(s)))
	}

	res := make([]Status, 48)
	for i, c := range s {
		res[i] = Status(c)
	}

	return res
}

// WithDate sets the date
func (b *ShutdownsBuilder) WithDate(date string) *ShutdownsBuilder {
	b.shutdowns.Date = date
	return b
}

// WithGroup adds a group with status items
func (b *ShutdownsBuilder) WithGroup(groupNum int, items ...Status) *ShutdownsBuilder {
	b.shutdowns.Groups[strconv.Itoa(groupNum)] = ShutdownGroup{
		Number: groupNum,
		Items:  items,
	}
	return b
}

// Build returns the constructed shutdowns
func (b *ShutdownsBuilder) Build() Shutdowns {
	return b.shutdowns
}
