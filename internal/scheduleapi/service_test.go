package scheduleapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Roma7-7-7/sso-notifier/internal/dal"
	"github.com/Roma7-7-7/sso-notifier/internal/providers"
)

type fakeProvider struct {
	today     dal.Shutdowns
	next      dal.Shutdowns
	hasNext   bool
	todayErr  error
	fetchedAt int
}

func (f *fakeProvider) Shutdowns(context.Context) (dal.Shutdowns, bool, error) {
	f.fetchedAt++
	return f.today, f.hasNext, f.todayErr
}

func (f *fakeProvider) ShutdownsNext(context.Context) (dal.Shutdowns, error) {
	return f.next, nil
}

var (
	evening = []dal.Period{{From: "22:00", To: "23:00"}, {From: "23:00", To: "23:30"}, {From: "23:30", To: "24:00"}}
	morning = []dal.Period{{From: "00:00", To: "00:30"}, {From: "00:30", To: "01:00"}, {From: "01:00", To: "02:00"}}
)

func day(date string, periods []dal.Period, items ...dal.Status) dal.Shutdowns {
	return dal.Shutdowns{
		Date:    date,
		Periods: periods,
		Groups:  map[string]dal.ShutdownGroup{"4": {Number: 4, Items: items}},
	}
}

func newTestService(t *testing.T, p Provider) *Service {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Kyiv")
	require.NoError(t, err)
	svc := NewService(p, loc, time.Minute, slog.New(slog.DiscardHandler))
	svc.now = func() time.Time { return time.Date(2025, 11, 4, 12, 0, 0, 0, loc) }
	return svc
}

func get(t *testing.T, svc *Service) (int, Snapshot) {
	t.Helper()
	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/schedule", nil))
	var snap Snapshot
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &snap))
	return rec.Code, snap
}

func TestService_NoDataYet(t *testing.T) {
	svc := newTestService(t, &fakeProvider{todayErr: errors.New("boom")})
	svc.refresh(t.Context())

	code, snap := get(t, svc)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "fetch today: boom", snap.LastError)
}

func TestService_MergesAcrossMidnight(t *testing.T) {
	p := &fakeProvider{
		today:   day("04.11.2025", evening, dal.ON, dal.OFF, dal.OFF),
		next:    day("05.11.2025", morning, dal.OFF, dal.OFF, dal.ON),
		hasNext: true,
	}
	svc := newTestService(t, p)
	svc.refresh(t.Context())

	code, snap := get(t, svc)
	require.Equal(t, http.StatusOK, code)

	got := snap.Groups["4"]
	require.Len(t, got, 3)
	assert.Equal(t, StatusOn, got[0].Status)
	assert.Equal(t, StatusOff, got[1].Status)
	assert.Equal(t, "23:00", got[1].Start.Format("15:04"))
	assert.Equal(t, "05.11 01:00", got[1].End.Format("02.01 15:04"))
	assert.Equal(t, StatusOn, got[2].Status)
}

func TestService_KeepsLastGoodDataOnFailure(t *testing.T) {
	p := &fakeProvider{today: day("04.11.2025", evening, dal.ON, dal.OFF, dal.OFF)}
	svc := newTestService(t, p)
	svc.refresh(t.Context())

	p.todayErr = errors.New("site down")
	svc.refresh(t.Context())

	code, snap := get(t, svc)
	assert.Equal(t, http.StatusOK, code)
	assert.NotEmpty(t, snap.Groups["4"])
	assert.Equal(t, "fetch today: site down", snap.LastError)
}

func TestService_Emergency(t *testing.T) {
	svc := newTestService(t, &fakeProvider{todayErr: providers.ErrEmergencyMode})
	svc.refresh(t.Context())

	code, snap := get(t, svc)
	assert.Equal(t, http.StatusOK, code)
	assert.True(t, snap.Emergency)
	assert.Empty(t, snap.Groups)
}
