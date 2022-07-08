// The Strava package implements methods to update Strava entries in response to receiving webhook events.
package strava

import (
	"context"
	"fmt"
	"log"
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

// activity struct holds only the data we want from the Strava API for an activity
type Activity struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Distance       float64   `json:"distance"`
	Type           string    `json:"type"`
	StartDate      time.Time `json:"start_date"`
	StartDateLocal time.Time `json:"start_date_local"`
	ElapsedTime    int64     `json:"elapsed_time"`
	ExternalID     string    `json:"external_id"`
	StartLatlng    []float64 `json:"start_latlng"`
	EndLatlng      []float64 `json:"end_latlng"`
	Trainer        bool      `json:"trainer"`
	Commute        bool      `json:"commute"`
	Private        bool      `json:"private"`
	WorkoutType    int       `json:"workout_type"`
	HideFromHome   bool      `json:"hide_from_leaderboard"`
	GearID         string    `json:"gear_id"`
	Description    string    `json:"description"`
}

type UpdatableActivity struct {
	Commute      bool   `json:"commute"`
	Trainer      bool   `json:"trainer"`
	HideFromHome bool   `json:"hide_from_leaderboard"`
	Description  string `json:"description"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	GearID       string `json:"gear_id"`
}

// https://developers.strava.com/docs/webhooks/#event-data
type WebhookPayload struct {
	SubscriptionID int64   `json:"subscription_id"`
	OwnerID        int64   `json:"owner_id"`
	ObjectID       int64   `json:"object_id"`
	ObjectType     string  `json:"object_type"`
	AspectType     string  `json:"aspect_type"`
	EventTime      int64   `json:"event_time"`
	Updates        updates `json:"updates"`
}

type updates struct {
	Title      string `json:"title,omitempty"`
	Type       string `json:"type,omitempty"`
	Private    string `json:"private,omitempty"`
	Authorized string `json:"authorized,omitempty"`
}

func GetActivity(c *client.Client, id int64) (*Activity, error) {
	var a Activity
	req, err := c.NewRequest("GET", fmt.Sprintf("/api/v3/activities/%d", id), nil)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = c.Do(context.Background(), req, &a)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &a, nil
}

func UpdateActivity(c *client.Client, id int64, ua *UpdatableActivity) (*Activity, error) {
	var a Activity
	req, err := c.NewRequest("PUT", fmt.Sprintf("/api/v3/activities/%d", id), ua)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	_, err = c.Do(context.Background(), req, &a)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &a, nil
}
