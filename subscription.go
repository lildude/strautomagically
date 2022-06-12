package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
)

// TODO: Rewrite me as I'm a hacky mess.
func existingSubscription() bool {
	u := fmt.Sprintf("%s/push_subscriptions?client_id=%s&client_secret=%s", "https://www.strava.com/api/v3", os.Getenv("STRAVA_CLIENT_ID"), os.Getenv("STRAVA_CLIENT_SECRET"))
	resp, err := http.Get(u)
	if err != nil {
		log.Printf("GET strava /push_subscriptions: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read push_subscriptions body: %s", err)
	}
	var subs []map[string]interface{}
	if err := json.Unmarshal(body, &subs); err != nil {
		log.Println(err)
	}
	if len(subs) == 0 {
		return false
	}
	if subs[0]["callback_url"] == os.Getenv("STRAVA_CALLBACK_URI") {
		return true
	}
	return false
}

func Subscribe() error {
	if existingSubscription() {
		return nil
	}

	resp, err := http.PostForm("https://www.strava.com/api/v3/push_subscriptions", url.Values{
		"client_id":     {os.Getenv("STRAVA_CLIENT_ID")},
		"client_secret": {os.Getenv("STRAVA_CLIENT_SECRET")},
		"callback_url":  {os.Getenv("STRAVA_CALLBACK_URI")},
		"verify_token":  {os.Getenv("STRAVA_VERIFY_TOKEN")},
	})
	if err != nil {
		log.Printf("POST strava /push_subscriptions: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		log.Printf("successfully subscribed to Strava activity feed")
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("failed to read push_subscriptions body: %s", err)
		return err
	}
	log.Printf("failed to subscribe to strava webhook: %s: %s", resp.Status, body)
	return err
}

// func Unsubscribe() {
// }
