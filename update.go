package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	strava "github.com/strava/go.strava"
)

// https://developers.strava.com/docs/webhooks/#event-data
type WebhookRequest struct {
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

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	token := os.Getenv("STRAVA_ACCESS_TOKEN")
	if token == "" {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte(`missing STRAVA_ACCESS_TOKEN`)); err != nil {
			log.Println(err)
		}
		return
	}

	var webhook WebhookRequest
	body, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal([]byte(body), &webhook); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err = w.Write([]byte(`failed to parse webhook request`)); err != nil {
			log.Println(err)
		}
		return
	}

	client := NewClient(token)
	service := strava.NewActivitiesService(client)
	activity, err := service.Get(webhook.ObjectID).Do()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err = w.Write([]byte(`failed to get activity`)); err != nil {
			log.Println(err)
		}
		return
	}

	// TODO: Move these to somewhere more configurable
	// Mute walks and set shoes
	if activity.Type == "Walk" {
		_, err := service.Update(activity.Id).Private(true).Gear("g10043849").Do()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
				log.Println(err)
			}
			return
		}
		log.Println("muted walk")
	}

	// Set Humane Burpees Title for WeightLifting activities between 3 & 7 minutes long
	if activity.Type == "WeightTraining" && activity.ElapsedTime >= 180 && activity.ElapsedTime <= 420 {
		_, err := service.Update(activity.Id).Private(true).Name("Humane Burpees").Do()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
				log.Println(err)
			}
			return
		}
		log.Println("set humane burpees title")
	}

	// Prefix name of rides with TR if external_id starts with traineroad and set gear to trainer
	if activity.Type == "Ride" && activity.ExternalId != "" && activity.ExternalId[0:7] == "trainerroad" {
		_, err := service.Update(activity.Id).Name("TR: " + activity.Name).Gear("b9880609").Do()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
				log.Println(err)
			}
			return
		}
		log.Println("prefixed name of ride with TR and set gear to trainer")
	}

	// Set gear to b9880609 if activity is a ride and external_id starts with zwift
	if activity.Type == "VirtualRide" && activity.ExternalId != "" && activity.ExternalId[0:5] == "zwift" {
		_, err := service.Update(activity.Id).Gear("b9880609").Do()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
				log.Println(err)
			}
			return
		}
		log.Println("set gear to trainer")
	}

	// Set gear to b10013574 if activity is a ride and not on trainer
	if activity.Type == "Ride" && !activity.Trainer {
		_, err := service.Update(activity.Id).Gear("b10013574").Do()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
				log.Println(err)
			}
			return
		}
		log.Println("set gear to bike")
	}

	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(`success`)); err != nil {
		log.Println(err)
	}
}