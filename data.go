package main

import (
	"sync"

	tele "gopkg.in/telebot.v3"
)

type Store struct {
	subscribers map[tele.ChatID]string

	mx sync.Mutex
}

func (s *Store) AddSubscriber(c tele.Context) bool {
	s.mx.Lock()
	defer s.mx.Unlock()
	if _, ok := s.subscribers[tele.ChatID(c.Chat().ID)]; ok {
		return false
	}

	s.subscribers[tele.ChatID(c.Chat().ID)] = ""
	return true
}

func (s *Store) GetWithDifferentHash(hash string) []tele.ChatID {
	s.mx.Lock()
	defer s.mx.Unlock()

	res := make([]tele.ChatID, 0)
	for k, v := range s.subscribers {
		if v != hash {
			res = append(res, k)
		}
	}
	return res
}

func (s *Store) UpdateHash(id tele.ChatID, hash string) {
	s.mx.Lock()
	defer s.mx.Unlock()
	s.subscribers[id] = hash
}

func (s *Store) Size() int {
	s.mx.Lock()
	defer s.mx.Unlock()
	return len(s.subscribers)
}

func NewStore() *Store {
	return &Store{
		subscribers: make(map[tele.ChatID]string),
	}
}
