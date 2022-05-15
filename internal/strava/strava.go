package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/lildude/strautomagically/internal/cache"
	"golang.org/x/oauth2"
)

var stravaConf *oauth2.Config

func init() {
	stravaConf = &oauth2.Config{
		ClientID:     os.Getenv("STRAVA_CLIENT_ID"),
		ClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://www.strava.com/oauth/authorize",
			TokenURL: "https://www.strava.com/oauth/token",
		},
		RedirectURL: os.Getenv("STRAVA_REDIRECT_URI"),
		Scopes:      []string{"activity:write,activity:read_all"},
	}
}

const (
	baseURL = "https://www.strava.com/api/v3"
)

// WebhookPayload is the payload sent from Strava when an activity is created, updated or deleted.
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
	Private    bool   `json:"private,omitempty"`
	Authorized bool   `json:"authorized,omitempty"`
}

// StravaActivity represents an activity that would be returned by the API query.
// This isn't the full activity object, but it's enough to get the data we need.
type StravaActivity struct {
	// The identifier provided at upload time
	ExternalId string `json:"external_id,omitempty"`
	// The name of the activity
	Name string `json:"name,omitempty"`
	// The activity's distance, in meters
	Distance float32 `json:"distance,omitempty"`
	// The activity's moving time, in seconds
	MovingTime int32 `json:"moving_time,omitempty"`
	// The activity's elapsed time, in seconds
	ElapsedTime int32 `json:"elapsed_time,omitempty"`
	// The activity type
	Type_ string `json:"type,omitempty"`
	// The time at which the activity was started.
	StartDate time.Time `json:"start_date,omitempty"`
	// The time at which the activity was started in the local timezone.
	StartDateLocal time.Time `json:"start_date_local,omitempty"`
	// Whether this activity was recorded on a training machine
	Trainer bool `json:"trainer,omitempty"`
	// Whether this activity is private
	Private bool `json:"private,omitempty"`
	// The activity's workout type
	WorkoutType int32 `json:"workout_type,omitempty"`
	// Whether the activity is muted
	HideFromHome bool `json:"hide_from_home,omitempty"`
	// The id of the gear for the activity
	GearId string `json:"gear_id,omitempty"`
	// The description of the activity
	Description string `json:"description,omitempty"`
}

func GetActivityById(id int64) (*StravaActivity, error) {
	// Get Access Token associated with user from cache
	authToken, err := getToken("strava_auth_token")
	if err != nil {
		log.Printf("unable to get token from redis: %s", err)
		return nil, err
	}
	// The Oauth2 library handles refreshing the token if it's expired.
	tokenSource := stravaConf.TokenSource(context.Background(), authToken)
	client := oauth2.NewClient(context.Background(), tokenSource)

	// Update our saved token
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("unable to refresh token: %s", err)
		return nil, err
	}
	if newToken.AccessToken != authToken.AccessToken {
		err = setToken("strava_auth_token", newToken)
		if err != nil {
			log.Printf("unable to store token: %s", err)
			return nil, err
		}
		log.Println("updated token")
	}
	// Get the activity
	url := fmt.Sprintf("%s/activities/%d", baseURL, id)
	resp, err := client.Get(url)
	if err != nil {
		return &StravaActivity{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return &StravaActivity{}, nil
	}
	_ = resp.Body.Close()

	// Unmarshal the JSON response into the StravaActivity struct
	var activity StravaActivity
	err = json.Unmarshal(body, &activity)
	if err != nil {
		return &StravaActivity{}, err
	}
	return &activity, nil
}

func getToken(key string) (*oauth2.Token, error) {
	cache, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		return nil, err
	}

	token := &oauth2.Token{}
	at, err := cache.Get(key)
	if err != nil {
		return nil, err
	}
	if at != "" {
		err = json.Unmarshal([]byte(fmt.Sprint(at)), &token)
		if err != nil {
			return nil, err
		}
	}
	return token, nil
}

func setToken(key string, token *oauth2.Token) error {
	cache, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		return err
	}

	t, err := json.Marshal(token)
	if err != nil {
		return err
	}
	return cache.Set(key, string(t))
}
