package providers

import "errors"

var (
	// ErrEmergencyMode indicates the power utility is in emergency mode and schedule is suspended.
	ErrEmergencyMode = errors.New("emergency mode - schedule suspended")

	// ErrNoScheduleAvailable indicates no schedule is available (no gsv div and no emergency article).
	ErrNoScheduleAvailable = errors.New("no schedule available")
)
