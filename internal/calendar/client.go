package calendar

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

const extendedPropertySource = "sso-notifier"

// Client wraps the Calendar API for listing, deleting, and inserting events.
type Client struct {
	svc        *calendar.Service
	calendarID string
	loc        *time.Location
}

// NewClient builds a Calendar API client using a service account JSON key file.
// Scope is calendar.events (create/read/delete). loc is used for time bounds (e.g. Europe/Kyiv).
func NewClient(ctx context.Context, credentialsPath, calendarID string, loc *time.Location) (*Client, error) {
	srv, err := calendar.NewService(ctx,
		option.WithCredentialsFile(credentialsPath),
		option.WithScopes(calendar.CalendarEventsScope),
	)
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}

	return &Client{
		svc:        srv,
		calendarID: calendarID,
		loc:        loc,
	}, nil
}

// ListOurEvents returns event IDs for events in [timeMin, timeMax] that have
// private extended property source=sso-notifier. Lists events in range then
// filters by ExtendedProperties.Private["source"] to avoid depending on
// PrivateExtendedProperty query support in the generated client.
func (c *Client) ListOurEvents(ctx context.Context, timeMin, timeMax time.Time) ([]string, error) {
	timeMinRFC := timeMin.In(c.loc).Format(time.RFC3339)
	timeMaxRFC := timeMax.In(c.loc).Format(time.RFC3339)

	call := c.svc.Events.List(c.calendarID).
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

// DeleteEvent removes the event by ID.
func (c *Client) DeleteEvent(ctx context.Context, eventID string) error {
	err := c.svc.Events.Delete(c.calendarID, eventID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("delete event %s: %w", eventID, err)
	}
	return nil
}

// InsertEvent creates an event with the given summary, start/end (RFC3339 in loc), colorId,
// and private extended property source=sso-notifier. Description is optional.
func (c *Client) InsertEvent(ctx context.Context, summary, startRFC3339, endRFC3339, colorID, description string) (*calendar.Event, error) {
	ev := &calendar.Event{
		Summary: summary,
		Start: &calendar.EventDateTime{
			DateTime: startRFC3339,
			TimeZone: "Europe/Kyiv",
		},
		End: &calendar.EventDateTime{
			DateTime: endRFC3339,
			TimeZone: "Europe/Kyiv",
		},
		ColorId: colorID,
		ExtendedProperties: &calendar.EventExtendedProperties{
			Private: map[string]string{"source": extendedPropertySource},
		},
	}
	if description != "" {
		ev.Description = description
	}

	created, err := c.svc.Events.Insert(c.calendarID, ev).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}
	return created, nil
}
