package dal

import (
	"fmt"
	"strconv"
	"time"

	"go.etcd.io/bbolt"
)

type (
	Clock interface {
		Now() time.Time
	}

	BoltDB struct {
		db    *bbolt.DB
		clock Clock
	}
)

func NewBoltDB(db *bbolt.DB, clock Clock) (*BoltDB, error) {
	return &BoltDB{
		db:    db,
		clock: clock,
	}, nil
}

func (s *BoltDB) Purge(chatID int64) error {
	if err := s.db.Update(func(tx *bbolt.Tx) error {
		subsBucket := tx.Bucket([]byte(subscriptionsBucket))
		if err := subsBucket.Delete(i64tob(chatID)); err != nil {
			return fmt.Errorf("delete subscriber with id=%d: %w", chatID, err)
		}

		if err := s.deleteNotificationStates(tx, chatID); err != nil {
			return fmt.Errorf("delete subscriber with id=%d: %w", chatID, err)
		}

		if err := s.deleteAlerts(tx, chatID); err != nil {
			return fmt.Errorf("delete subscriber with id=%d: %w", chatID, err)
		}

		if err := s.deleteEmergencyNotification(tx, chatID); err != nil {
			return fmt.Errorf("delete emergency notification for id=%d: %w", chatID, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (s *BoltDB) Close() error {
	return s.db.Close()
}

func i64tob(id int64) []byte {
	return []byte(strconv.FormatInt(id, 10))
}
