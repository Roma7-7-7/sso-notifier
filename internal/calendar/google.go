package calendar

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const (
	extendedPropertySource = "sso-notifier"
	reminderMinutes        = 15
)

// Google wraps the Calendar API for listing, deleting, and inserting events.
type Google struct {
	svc *calendar.Service
}

// NewGoogle builds a Calendar API client using a service account JSON key file.
// Scope is calendar.events (create/read/delete). loc is used for time bounds (e.g. Europe/Kyiv).
func NewGoogle(ctx context.Context, credentialsPath string) (*Google, error) {
	srv, err := calendar.NewService(ctx,
		option.WithAuthCredentialsFile(option.ServiceAccount, credentialsPath),
		option.WithScopes(calendar.CalendarEventsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}

	return &Google{
		svc: srv,
	}, nil
}

// ListOurEvents returns event IDs for events in [timeMin, timeMax] that have
// private extended property source=sso-notifier. Lists events in range then
// filters by ExtendedProperties.Private["source"] to avoid depending on
// PrivateExtendedProperty query support in the generated client.
func (c *Google) ListOurEvents(ctx context.Context, calendarID string, timeMin, timeMax time.Time) ([]string, error) {
	timeMinRFC := timeMin.Format(time.RFC3339)
	timeMaxRFC := timeMax.Format(time.RFC3339)

	call := c.svc.Events.List(calendarID).
		Context(ctx).
		TimeMin(timeMinRFC).
		TimeMax(timeMaxRFC).
		SingleEvents(true)

	var ids []string
	err := call.Pages(ctx, func(events *calendar.Events) error {
		for _, e := range events.Items {
			if e.Id == "" {
				continue
			}
			if e.ExtendedProperties != nil && e.ExtendedProperties.Private != nil {
				if e.ExtendedProperties.Private["source"] == extendedPropertySource {
					ids = append(ids, e.Id)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return ids, nil
}

// InsertEvent creates an event with the given summary, start/end, colorId,
// and private extended property source=sso-notifier. Description is optional.
func (c *Google) InsertEvent(ctx context.Context, calendarID, summary string, start, end time.Time, params EventParams) (string, error) {
	ev := &calendar.Event{
		Summary: summary,
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: start.Location().String(),
		},
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: end.Location().String(),
		},
		ColorId: params.ColorID,
		ExtendedProperties: &calendar.EventExtendedProperties{
			Private: map[string]string{"source": extendedPropertySource},
		},
		Description: params.Description,
		Reminders: &calendar.EventReminders{
			Overrides:       []*calendar.EventReminder{{Method: "popup", Minutes: reminderMinutes}},
			ForceSendFields: []string{"UseDefault", "Overrides"},
		},
	}

	created, err := c.svc.Events.Insert(calendarID, ev).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("insert event: %w", err)
	}
	return created.Id, nil
}

// DeleteEvent removes the event by ID.
func (c *Google) DeleteEvent(ctx context.Context, calendarID, eventID string) error {
	err := c.svc.Events.Delete(calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("delete event %s: %w", eventID, err)
	}
	return nil
}
