package scheduleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
)

const fetchTimeout = 30 * time.Second

type Provider interface {
	Shutdowns(ctx context.Context) (dal.Shutdowns, bool, error)
	ShutdownsNext(ctx context.Context) (dal.Shutdowns, error)
}

// Snapshot is the API response. Data is kept from the last successful fetch, so clients
// should compare FetchedAt with the current time to detect staleness.
type Snapshot struct {
	FetchedAt     time.Time             `json:"fetched_at"`
	LastAttemptAt time.Time             `json:"last_attempt_at"`
	LastError     string                `json:"last_error,omitempty"`
	Emergency     bool                  `json:"emergency"`
	Groups        map[string][]Interval `json:"groups"`
}

type Service struct {
	provider Provider
	loc      *time.Location
	interval time.Duration
	log      *slog.Logger
	now      func() time.Time

	mu   sync.RWMutex
	snap *Snapshot
	last time.Time
	err  string
}

func NewService(provider Provider, loc *time.Location, interval time.Duration, log *slog.Logger) *Service {
	return &Service{provider: provider, loc: loc, interval: interval, log: log, now: func() time.Time { return time.Now().In(loc) }}
}

// Run refreshes immediately and then on every interval until ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	s.refresh(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func (s *Service) refresh(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	groups, err := s.fetch(ctx)
	now := s.now().Truncate(time.Second)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.last = now

	switch {
	case err == nil:
		s.snap = &Snapshot{FetchedAt: now, Groups: groups}
		s.err = ""
		s.log.InfoContext(ctx, "Schedule refreshed", "groups", len(groups))
	case errors.Is(err, providers.ErrEmergencyMode):
		s.snap = &Snapshot{FetchedAt: now, Emergency: true, Groups: map[string][]Interval{}}
		s.err = ""
		s.log.WarnContext(ctx, "Emergency mode, schedule suspended")
	default:
		s.err = err.Error()
		s.log.ErrorContext(ctx, "Failed to refresh schedule", "error", err)
	}
}

func (s *Service) fetch(ctx context.Context) (map[string][]Interval, error) {
	today, nextAvailable, err := s.provider.Shutdowns(ctx)
	if err != nil && !errors.Is(err, providers.ErrCheckNextDayAvailability) {
		return nil, fmt.Errorf("fetch today: %w", err)
	}
	if err != nil {
		nextAvailable = false
	}

	days := []dal.Shutdowns{today}
	if nextAvailable {
		next, err := s.provider.ShutdownsNext(ctx)
		if err != nil {
			// today's data is still useful, so don't fail the whole refresh
			s.log.WarnContext(ctx, "Failed to fetch tomorrow's schedule", "error", err)
		} else if next.Date != today.Date {
			days = append(days, next)
		}
	}

	all := make(map[string][]Interval)
	for _, d := range days {
		intervals, err := toIntervals(d, s.loc)
		if err != nil {
			return nil, fmt.Errorf("convert schedule date=%s: %w", d.Date, err)
		}
		for k, v := range intervals {
			all[k] = append(all[k], v...)
		}
	}

	for k, v := range all {
		sort.Slice(v, func(i, j int) bool { return v[i].Start.Before(v[j].Start) })
		all[k] = merge(v)
	}

	return all, nil
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/schedule", s.handleSchedule)

	return mux
}

func (s *Service) handleSchedule(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	var out Snapshot
	have := s.snap != nil
	if have {
		out = *s.snap
	}
	out.LastAttemptAt = s.last
	out.LastError = s.err
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if !have {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	if err := json.NewEncoder(w).Encode(out); err != nil {
		s.log.Error("Failed to write response", "error", err)
	}
}
