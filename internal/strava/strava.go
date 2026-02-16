// Package strava implements methods to update Strava entries in response to receiving webhook events.
package strava

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lildude/strautomagically/internal/client"
	"golang.org/x/oauth2"
)

var (
	BaseURL     = "https://www.strava.com/api/v3"
	OauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		ClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/oauth/token",
		},
		RedirectURL: os.Getenv("STRAVA_REDIRECT_URI"),
		Scopes:      []string{"activity:write,activity:read_all"},
	}
)

// Activity struct holds only the data we want from the Strava API for an activity.
type Activity struct {
	Commute        bool      `json:"commute"`
	Description    string    `json:"description"`
	Distance       float64   `json:"distance"`
	ElapsedTime    int64     `json:"elapsed_time"`
	EndLatlng      []float64 `json:"end_latlng"`
	ExternalID     string    `json:"external_id"`
	GearID         string    `json:"gear_id"`
	HideFromHome   bool      `json:"hide_from_home"`
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Private        bool      `json:"private"`
	StartDate      time.Time `json:"start_date"`
	StartDateLocal time.Time `json:"start_date_local"`
	StartLatlng    []float64 `json:"start_latlng"`
	Trainer        bool      `json:"trainer"`
	Type           string    `json:"type"`
	WorkoutType    int       `json:"workout_type"`
}

type UpdatableActivity struct {
	Commute      bool   `json:"commute,omitempty"`
	Description  string `json:"description,omitempty"`
	GearID       string `json:"gear_id,omitempty"`
	HideFromHome bool   `json:"hide_from_home,omitempty"`
	Name         string `json:"name,omitempty"`
	Private      bool   `json:"private,omitempty"`
	Trainer      bool   `json:"trainer,omitempty"`
	Type         string `json:"type,omitempty"`
}

type WebhookPayload struct {
	AspectType     string  `json:"aspect_type"`
	EventTime      int64   `json:"event_time"`
	ObjectID       int64   `json:"object_id"`
	ObjectType     string  `json:"object_type"`
	OwnerID        int64   `json:"owner_id"`
	SubscriptionID int64   `json:"subscription_id"`
	Updates        updates `json:"updates"`
}

type updates struct {
	Authorized string `json:"authorized,omitempty"`
	Private    string `json:"private,omitempty"`
	Title      string `json:"title,omitempty"`
	Type       string `json:"type,omitempty"`
}

func GetActivity(ctx context.Context, c *client.Client, id int64) (*Activity, error) {
	var a Activity
	req, err := c.NewRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v3/activities/%d", id), nil)
	if err != nil {
		return nil, fmt.Errorf("creating get activity request: %w", err)
	}

	resp, err := c.Do(req, &a)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("getting activity %d: %w", id, err)
	}

	return &a, nil
}

func UpdateActivity(ctx context.Context, c *client.Client, id int64, ua *UpdatableActivity) (*Activity, error) {
	var a Activity
	req, err := c.NewRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v3/activities/%d", id), ua)
	if err != nil {
		return nil, fmt.Errorf("creating update activity request: %w", err)
	}

	resp, err := c.Do(req, &a)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("updating activity %d: %w", id, err)
	}

	return &a, nil
}
