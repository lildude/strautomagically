package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

// TODO: Rewrite me as I'm a hacky mess.
func existingSubscription() bool {
	u := fmt.Sprintf("%s/push_subscriptions?client_id=%s&client_secret=%s",
		"https://www.strava.com/api/v3",
		os.Getenv("STRAVA_CLIENT_ID"),
		os.Getenv("STRAVA_CLIENT_SECRET"))
	resp, err := http.Get(u) //nolint:gosec,noctx
	if err != nil {
		log.Println("[INFO] GET strava /push_subscriptions:", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("[ERROR] failed to read push_subscriptions body:", err)
	}
	var subs []map[string]interface{}
	if err := json.Unmarshal(body, &subs); err != nil {
		log.Println("[ERROR]", err)
	}
	if len(subs) == 0 {
		return false
	}
	if subs[0]["callback_url"] == os.Getenv("STRAVA_CALLBACK_URI") {
		return true
	}
	return false
}

func Subscribe() (bool, error) {
	// TODO: Detect if this is our sub and if so, delete it first.
	if existingSubscription() {
		return false, nil
	}

	resp, err := http.PostForm("https://www.strava.com/api/v3/push_subscriptions", url.Values{ //nolint:noctx
		"client_id":     {os.Getenv("STRAVA_CLIENT_ID")},
		"client_secret": {os.Getenv("STRAVA_CLIENT_SECRET")},
		"callback_url":  {os.Getenv("STRAVA_CALLBACK_URI")},
		"verify_token":  {os.Getenv("STRAVA_VERIFY_TOKEN")},
	})
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return true, nil
	}

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	return true, err
}

// func Unsubscribe() {
// }
