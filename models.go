package main

import (
	"errors"
)

var ErrBlockedByUser = errors.New("bot is blocked by user")

type Subscriber struct {
	ChatID int64 `json:"chat_id"`
}

type Notification struct {
	ID     int        `json:"id"`
	Target Subscriber `json:"target"`
	Msg    string     `json:"message"`
}
