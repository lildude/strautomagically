// Package calendarevent implements methods to get events from ical feeds.
package calendarevent

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/apognu/gocal"
)

type Event struct {
	Summary     string
	Description string
	Start       time.Time
	End         time.Time
}

type CalendarEventGetter interface {
	GetCalendarEvent(time.Time) (*Event, error)
}

type CalendarService struct {
	Client  HTTPClient
	BaseURL string
	CalID   string
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewCalendarService(client HTTPClient, baseURL, calID string) *CalendarService {
	return &CalendarService{
		Client:  client,
		BaseURL: baseURL,
		CalID:   calID,
	}
}

// GetCalendarEvent returns a single event for the date passed in.
func (cs CalendarService) GetCalendarEvent(start time.Time) (*Event, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", cs.BaseURL, cs.CalID), http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := cs.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	c := gocal.NewParser(resp.Body)
	c.Start, c.End = &start, &start

	err = c.Parse()
	if err != nil {
		return nil, err
	}

	var events []Event
	for i := 0; i < len(c.Events); i++ {
		component := c.Events[i]
		events = append(events, Event{
			Summary:     parseSummary(component.Summary),
			Description: component.Description,
			Start:       *component.Start,
			End:         *component.End,
		})
	}

	// We only want one entry for the date passed in.
	if len(events) > 0 {
		return &events[0], nil
	}

	return nil, nil
}

// ParseSummary parses the summary field of a TrainerRoad event and returns just the workout name.
func parseSummary(summary string) string {
	return summary[strings.Index(summary, "-")+2:]
}
