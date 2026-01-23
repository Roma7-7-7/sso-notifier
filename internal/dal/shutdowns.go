package dal

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

const (
	shutdownsBucket = "shutdowns"

	ON    Status = "Y"
	OFF   Status = "N"
	MAYBE Status = "M"

	emergencyStateKey = "_emergency"
)

type (
	Status string

	Date struct {
		Year  int
		Month time.Month
		Day   int
	}

	Shutdowns struct {
		Date    string                   `json:"date"`
		Periods []Period                 `json:"periods"`
		Groups  map[string]ShutdownGroup `json:"groups"`
	}

	Period struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	ShutdownGroup struct {
		Number int      `json:"number"`
		Items  []Status `json:"items"`
	}

	EmergencyState struct {
		Active    bool      `json:"active"`
		StartedAt time.Time `json:"started_at"`
	}
)

func (d Date) ToKey() string {
	return fmt.Sprintf("%d-%02d-%02d", d.Year, d.Month, d.Day)
}

func DateByTime(now time.Time) Date {
	return Date{
		Year:  now.Year(),
		Month: now.Month(),
		Day:   now.Day(),
	}
}

func TomorrowDateByTime(now time.Time) Date {
	tomorrow := now.Add(24 * time.Hour) //nolint:mnd // 1 day
	return Date{
		Year:  tomorrow.Year(),
		Month: tomorrow.Month(),
		Day:   tomorrow.Day(),
	}
}

func (s *BoltDB) GetShutdowns(d Date) (Shutdowns, bool, error) {
	var res Shutdowns
	found := false

	err := s.db.View(func(tx *bbolt.Tx) error {
		data := tx.Bucket([]byte(shutdownsBucket)).Get([]byte(d.ToKey()))
		if data == nil {
			return nil
		}
		found = true
		return json.Unmarshal(data, &res)
	})

	return res, found, err
}

func (s *BoltDB) PutShutdowns(d Date, t Shutdowns) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("marshal shutdowns table: %w", err)
		}
		return tx.Bucket([]byte(shutdownsBucket)).Put([]byte(d.ToKey()), data)
	})
}

func (s *BoltDB) GetEmergencyState() (EmergencyState, error) {
	var state EmergencyState

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(shutdownsBucket))
		if b == nil {
			return nil
		}

		data := b.Get([]byte(emergencyStateKey))
		if data == nil {
			return nil
		}

		return json.Unmarshal(data, &state)
	})

	return state, err
}

func (s *BoltDB) SetEmergencyState(state EmergencyState) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(shutdownsBucket))
		if b == nil {
			return fmt.Errorf("bucket %s not found", shutdownsBucket)
		}

		data, err := json.Marshal(&state)
		if err != nil {
			return fmt.Errorf("marshal emergency state: %w", err)
		}

		return b.Put([]byte(emergencyStateKey), data)
	})
}
