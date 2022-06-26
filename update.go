package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/antihax/optional"
	"github.com/lildude/strautomagically/internal/weather"
	"github.com/lildude/strava-swagger"
)

// https://developers.strava.com/docs/webhooks/#event-data
type webhookPayload struct {
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

func updateHandler(w http.ResponseWriter, r *http.Request) {
	var webhook webhookPayload
	body, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal([]byte(body), &webhook); err != nil {
		log.Println("unable to unmarshal webhook payload:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// We only react to new activities for now
	if webhook.AspectType != "create" {
		w.WriteHeader(http.StatusOK)
		log.Println("ignoring non-create webhook")
		return
	}

	client, err := newStravaClient()
	if err != nil {
		log.Println("unable to create strava client", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	activity, _, err := client.ActivitiesApi.GetActivityById(ctx, webhook.ObjectID, nil)
	if err != nil {
		log.Println("Unable to get activity", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Activity:", activity.Name)

	var update strava.UpdatableActivity
	msg := "nothing to do"

	// TODO: Move these to somewhere more configurable
	// Mute walks and set shoes
	if *activity.Type_ == "Walk" {
		update.HideFromHome = true
		update.GearId = "g10043849"
		msg = "muted walk"
	}
	// Set Humane Burpees Title for WeightLifting activities between 3 & 7 minutes long
	if *activity.Type_ == "WeightTraining" && activity.ElapsedTime >= 180 && activity.ElapsedTime <= 420 {
		update.HideFromHome = true
		update.Name = "Humane Burpees"
		msg = "set humane burpees title"
	}
	// Prefix name of rides with TR if external_id starts with traineroad and set gear to trainer
	if *activity.Type_ == "Ride" && activity.ExternalId != "" && activity.ExternalId[0:7] == "trainerroad" {
		update.Name = "TR: " + activity.Name
		update.GearId = "b9880609"
		msg = "prefixed name of ride with TR and set gear to trainer"
	}
	// Set gear to b9880609 if activity is a ride and external_id starts with zwift
	if *activity.Type_ == "VirtualRide" && activity.ExternalId != "" && activity.ExternalId[0:5] == "zwift" {
		update.GearId = "b9880609"
		msg = "set gear to trainer"
	}
	// Set gear to b10013574 if activity is a ride and not on trainer
	if *activity.Type_ == "Ride" && !activity.Trainer {
		update.GearId = "b10013574"
		msg = "set gear to bike"
	}
	// Set title for specific Pete's Plan workouts and warmups
	var title string
	if *activity.Type_ == "Rowing" {
		switch activity.Name {
		case "v250m/1:30r...7 row":
			title = "Speed Pyramid Row w/ 1.5' RI per 250m work"
		case "8x500m/3:30r row":
			title = "8x 500m w/ 3.5' RI Row"
		case "5x1500m/5:00r row":
			title = "5x 1500m w/ 5' RI Row"
		case "4x2000m/5:00r row":
			title = "4x 2000m w/5' RI Row"
		case "4x1000m/5:00r row":
			title = "4x 1000m /5' RI Row"
		case "v3000m/5:00r...3 row":
			title = "Waterfall of 3k, 2.5k, 2k w/ 5' RI Row"
		case "5:00 row":
			title = "Warm-up Row"
			update.HideFromHome = true
		}
		update.Name = title
		msg = fmt.Sprintf("set title to %s", title)
	}
	// Add weather for activity if no GPS data - assumes we were at home
	if len(*activity.StartLatlng) == 0 {
		weather := weather.GetWeather(activity.StartDateLocal, activity.ElapsedTime)
		if weather != "" && !strings.Contains(activity.Description, "AQI") {
			update.Description = fmt.Sprintf("%s\n\n%s", activity.Description, weather)
			msg = "added weather"
		}
	}

	_, _, err = client.ActivitiesApi.UpdateActivityById(
		ctx, activity.Id,
		&strava.ActivitiesApiUpdateActivityByIdOpts{Body: optional.NewInterface(update)},
	)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
			log.Println(err)
		}
		return
	}
	log.Println(msg)

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(`success`)); err != nil {
		log.Println(err)
	}
}
