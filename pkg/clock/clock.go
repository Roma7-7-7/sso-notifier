package clock

import "time"

type Clock struct {
}

func New() *Clock {
	return &Clock{}
}

func (c *Clock) Now() time.Time {
	return time.Now()
}

type Mock struct {
	value time.Time
}

func NewMock(value time.Time) *Mock {
	return &Mock{
		value: value,
	}
}

func (m *Mock) Now() time.Time {
	return m.value
}

func (m *Mock) Set(t time.Time) {
	m.value = t
}
