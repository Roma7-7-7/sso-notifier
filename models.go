package main

import (
	"errors"
)

var ErrBlockedByUser = errors.New("bot is blocked by user")

type Subscription struct {
	ChatID int64             `json:"chat_id"`
	Groups map[string]string `json:"groups"`
}

type Notification struct {
	ID     int    `json:"id"`
	Target int64  `json:"target"`
	Msg    string `json:"message"`
}
