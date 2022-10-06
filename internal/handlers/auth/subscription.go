package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	log "github.com/sirupsen/logrus"
)

// TODO: Rewrite me as I'm a hacky mess.
func existingSubscription() bool {
	u := fmt.Sprintf("%s/push_subscriptions?client_id=%s&client_secret=%s",
		"https://www.strava.com/api/v3",
		os.Getenv("STRAVA_CLIENT_ID"),
		os.Getenv("STRAVA_CLIENT_SECRET"))
	resp, err := http.Get(u) //nolint:gosec,noctx
	if err != nil {
		log.Errorf("GET strava /push_subscriptions: %s", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to read push_subscriptions body: %s", err)
	}
	var subs []map[string]interface{}
	if err := json.Unmarshal(body, &subs); err != nil {
		log.Errorln(err)
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
	// TODO: Detect if this is our sub and if so, delete it first.
	if existingSubscription() {
		log.Infoln("existing subscription found, skipping")
		return nil
	}

	resp, err := http.PostForm("https://www.strava.com/api/v3/push_subscriptions", url.Values{ //nolint:noctx
		"client_id":     {os.Getenv("STRAVA_CLIENT_ID")},
		"client_secret": {os.Getenv("STRAVA_CLIENT_SECRET")},
		"callback_url":  {os.Getenv("STRAVA_CALLBACK_URI")},
		"verify_token":  {os.Getenv("STRAVA_VERIFY_TOKEN")},
	})
	if err != nil {
		log.Errorf("POST strava /push_subscriptions: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		log.Infoln("successfully subscribed to Strava activity feed")
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to read push_subscriptions body: %s", err)
		return err
	}
	log.Errorf("failed to subscribe to strava webhook: %s: %s", resp.Status, body)
	return err
}

// func Unsubscribe() {
// }
