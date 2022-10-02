package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/client"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/lildude/strautomagically/internal/weather"
	"golang.org/x/oauth2"
)

func UpdateHandler(w http.ResponseWriter, r *http.Request) { //nolint:funlen
	var webhook strava.WebhookPayload
	if r.Body == nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &webhook); err != nil {
		log.Println("unable to unmarshal webhook payload:", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// We only react to new activities for now
	if webhook.AspectType != "create" {
		w.WriteHeader(http.StatusOK)
		log.Println("ignoring non-create webhook")
		return
	}

	rcache, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Printf("unable to create redis cache: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// See if we've seen this activity before
	aid, err := rcache.Get("strava_activity")
	if err != nil {
		log.Printf("unable to get activity id from cache: %s", err)
	}
	// Convert aid to int
	s, _ := aid.(string)
	aidInt, _ := strconv.ParseInt(s, 10, 64)

	if aidInt == webhook.ObjectID {
		w.WriteHeader(http.StatusOK)
		log.Println("ignoring repeat event")
		return
	}

	// Create the OAuth http.Client
	ctx := context.Background()
	authToken := &oauth2.Token{}
	err = rcache.GetJSON("strava_auth_token", &authToken)
	if err != nil {
		log.Printf("unable to get token: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// The Oauth2 library handles refreshing the token if it's expired.
	ts := strava.OauthConfig.TokenSource(context.Background(), authToken)
	tc := oauth2.NewClient(ctx, ts)
	surl, _ := url.Parse(strava.BaseURL)

	newToken, err := ts.Token()
	if err != nil {
		log.Printf("unable to refresh token: %s", err)
		return
	}
	if newToken.AccessToken != authToken.AccessToken {
		err = rcache.SetJSON("strava_auth_token", newToken)
		if err != nil {
			log.Printf("unable to store token: %s", err)
			return
		}
		log.Println("updated token")
	}

	sc := client.NewClient(surl, tc)

	activity, err := strava.GetActivity(sc, webhook.ObjectID)
	if err != nil {
		log.Printf("unable to get activity: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	log.Printf("Activity:%s (%d)", activity.Name, activity.ID)

	baseURL := &url.URL{Scheme: "https", Host: "api.openweathermap.org", Path: "/data/3.0/onecall"}
	wclient := client.NewClient(baseURL, nil)
	update := constructUpdate(wclient, activity)

	if (&strava.UpdatableActivity{}) != update {
		var updated *strava.Activity
		updated, err = strava.UpdateActivity(sc, webhook.ObjectID, update)
		if err != nil {
			log.Printf("unable to update activity: %s", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		log.Printf("Updated activity:%s (%d) Hidden: %t", updated.Name, updated.ID, updated.HideFromHome)

		// Cache activity ID if we've succeeded
		err = rcache.Set("strava_activity", webhook.ObjectID)
		if err != nil {
			log.Printf("unable to cache activity id: %s", err)
		}
	}

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(`success`)); err != nil {
		log.Println(err)
	}
}

func constructUpdate(wclient *client.Client, activity *strava.Activity) *strava.UpdatableActivity { //nolint:funlen,gocyclo
	var update strava.UpdatableActivity
	var title string
	var msg string
	const trainer = "b9880609" // Tacx Neo 2T Turbo
	const bike = "b10013574"   // Dolan Tuono Disc
	const shoes = "g10043849"  // No name, Not running shoes

	// TODO: Move these to somewhere more configurable
	switch activity.Type {
	// I'll never handcycle. This is used for testing only
	case "Handcycle":
		break

	case "Ride":
		// Prefix name of rides with TR if external_id starts with traineroad and set gear to trainer
		if activity.ExternalID != "" && activity.ExternalID[0:11] == "trainerroad" {
			update.Name = "TR: " + activity.Name
			update.GearID = trainer
			update.Trainer = true
			msg = "prefixed name of ride with TR and set gear to trainer"
		}
		// Set gear to b10013574 if activity is a ride and not on trainer
		if !activity.Trainer {
			update.GearID = bike
			msg = "set gear to bike"
		}

	case "Rowing":
		// Workouts created in ErgZone will have the name in the first line of the description
		lines := strings.Split(activity.Description, "\n")
		if len(lines) > 0 {
			// We only want the first line if the description contains the https://erg.zone URL
			if strings.Contains(activity.Description, "https://erg.zone") {
				title = lines[0]
				update.Description = "\n"
			}
		}

		// Fallback to the name if there is no description or it doesn't contain "https://erg.zone"
		if title == "" {
			switch activity.Name {
			case "v250m/1:30r...7 row", "v5:00/1:00r...15 row":
				title = "Speed Pyramid Row w/ 1.5' Active RI per 250m work"
			case "8x500m/3:30r row", "v5:00/1:00r...17 row":
				title = "8x 500m w/ 3.5' Active RI Row"
			case "5x1500m/5:00r row":
				title = "5x 1500m w/ 5' RI Row"
			case "4x2000m/5:00r row", "v5:00/1:00r...9 row":
				title = "4x 2000m w/5' Active RI Row"
			case "4x1000m/5:00r row":
				title = "4x 1000m /5' RI Row"
			case "v3000m/5:00r...3 row", "v5:00/1:00r...7 row":
				title = "Waterfall of 3k, 2.5k, 2k w/ 5' Active RI Row"
			case "5:00 row":
				title = "Warm-up Row"
				update.HideFromHome = true
			}
		}
		update.Name = title
		if title != "" {
			msg = fmt.Sprintf("set title to %s", title)
		}

	case "Run":
		break

	case "VirtualRide":
		// Set gear to trainer if activity is a ride and external_id starts with zwift
		if activity.ExternalID != "" && activity.ExternalID[0:5] == "zwift" {
			update.GearID = trainer
			update.Trainer = true
			msg = "set gear to trainer"
		}
	case "Walk":
		// Mute walks and set shoes
		update.HideFromHome = true
		update.GearID = shoes
		msg = "muted walk"
	case "WeightTraining":
		// Set Humane Burpees Title for WeightLifting activities between 3 & 7 minutes long
		if activity.ElapsedTime >= 180 && activity.ElapsedTime <= 420 {
			update.HideFromHome = true
			update.Name = "Humane Burpees"
			msg = "set humane burpees title"
		}
	}

	// Add weather for activity if no GPS data - assumes we were at home
	if len(activity.StartLatlng) == 0 {
		if !strings.Contains(activity.Description, "AQI") {
			w, err := weather.GetWeatherLine(wclient, activity.StartDateLocal, int32(activity.ElapsedTime))
			if err != nil {
				log.Printf("unable to get weather: %s", err)
			}
			if w != "" {
				if activity.Description != "" && update.Description != "\n" {
					update.Description = activity.Description + "\n\n"
				}
				update.Description += w
				if msg != "" {
					msg += " & "
				}
				msg += "added weather"
			}
		}
	}

	if msg == "" {
		msg = "nothing to do"
	}

	log.Println(msg)

	return &update
}
