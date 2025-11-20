package clock

import "time"

type Clock struct {
	loc *time.Location
}

func New() *Clock {
	return &Clock{}
}

func NewWithLocation(loc *time.Location) *Clock {
	return &Clock{loc: loc}
}

func (c *Clock) Now() time.Time {
	now := time.Now()
	if c.loc != nil {
		now = now.In(c.loc)
	}
	return now
}

type Mock struct {
	value func() time.Time
}

func NewMock(value time.Time) *Mock {
	return &Mock{
		value: func() time.Time {
			return value
		},
	}
}

func NewMockF(value func() time.Time) *Mock {
	return &Mock{
		value: value,
	}
}

func (m *Mock) Now() time.Time {
	return m.value()
}

func (m *Mock) Set(t time.Time) {
	m.value = func() time.Time {
		return t
	}
}

func (m *Mock) SetF(value func() time.Time) {
	m.value = value
}
