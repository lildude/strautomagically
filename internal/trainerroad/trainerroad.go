// Package trainerroad implements methods download TrainerRoad calendar entries for the ical feed.
package trainerroad

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/apognu/gocal"
)

var (
	BaseURL = "https://api.trainerroad.com/v1/calendar/ics"
	CalID   = os.Getenv("TRAINERROAD_CAL_ID")
)

type Event struct {
	Summary     string
	Description string
	Start       time.Time
	End         time.Time
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// GetCalendarEvent returns a single event for the date passed in.
func GetCalendarEvent(client HTTPClient, start time.Time) (*Event, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", BaseURL, CalID), http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
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
