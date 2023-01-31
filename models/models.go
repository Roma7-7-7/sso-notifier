package models

import (
	"bytes"
	"errors"
	"fmt"
)

var ErrSubscriptionsLimitReached = errors.New("subscriptions limit reached")

type Subscription struct {
	ChatID int64             `json:"chat_id"`
	Groups map[string]string `json:"groups"`
}

type Status string

const (
	ON    Status = "Y"
	OFF   Status = "N"
	MAYBE Status = "M"
)

func (s Status) Hash() string {
	return string(s)
}

type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ShutdownGroup struct {
	Number int
	Items  []Status
}

func (g ShutdownGroup) Hash() string {
	var buf bytes.Buffer
	for _, i := range g.Items {
		buf.WriteString(i.Hash())
	}
	return buf.String()
}

func (g ShutdownGroup) Validate(expectedItemsNum int) error {
	if g.Number < 1 {
		return fmt.Errorf("invalid shutdown group number=%d", g.Number)
	}
	if len(g.Items) != expectedItemsNum {
		return fmt.Errorf("invalid shutdown group items size; expected=%d but actual=%d", expectedItemsNum, len(g.Items))
	}
	return nil
}

type ShutdownsTable struct {
	ID      string                   `json:"id"`
	Date    string                   `json:"date"`
	Periods []Period                 `json:"periods"`
	Groups  map[string]ShutdownGroup `json:"groups"`
}

func (s ShutdownsTable) Validate() error {
	if s.Date == "" {
		return fmt.Errorf("invalid shutdowns table date=%s", s.Date)
	}
	if len(s.Periods) == 0 {
		return fmt.Errorf("shutdowns table periods list is empty")
	}
	for _, g := range s.Groups {
		if err := g.Validate(len(s.Periods)); err != nil {
			return fmt.Errorf("invalid shutdowns table group=%v: %w", g, err)
		}
	}
	return nil
}

type Notification struct {
	ID     int    `json:"id"`
	Target int64  `json:"target"`
	Msg    string `json:"message"`
}
