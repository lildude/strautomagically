package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/antihax/optional"
	"github.com/lildude/strautomagically/internal/strava-swagger"
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
	Private    bool   `json:"private,omitempty"`
	Authorized bool   `json:"authorized,omitempty"`
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
