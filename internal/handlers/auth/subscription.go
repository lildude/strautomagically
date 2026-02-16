package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// TODO: Rewrite me as I'm a hacky mess.
func existingSubscription(ctx context.Context) bool {
	u := fmt.Sprintf("%s/push_subscriptions?client_id=%s&client_secret=%s",
		"https://www.strava.com/api/v3",
		os.Getenv("STRAVA_CLIENT_ID"),
		os.Getenv("STRAVA_CLIENT_SECRET"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		slog.Error("creating push_subscriptions request", "error", err)
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error("GET strava /push_subscriptions", "error", err)
		return false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("reading push_subscriptions body", "error", err)
		return false
	}
	var subs []map[string]any
	if err := json.Unmarshal(body, &subs); err != nil {
		slog.Error("unmarshaling subscriptions", "error", err)
		return false
	}
	if len(subs) == 0 {
		return false
	}
	return subs[0]["callback_url"] == os.Getenv("STRAVA_CALLBACK_URI")
}

func Subscribe(ctx context.Context) (bool, error) {
	// TODO: Detect if this is our sub and if so, delete it first.
	if existingSubscription(ctx) {
		return false, nil
	}

	form := url.Values{
		"client_id":     {os.Getenv("STRAVA_CLIENT_ID")},
		"client_secret": {os.Getenv("STRAVA_CLIENT_SECRET")},
		"callback_url":  {os.Getenv("STRAVA_CALLBACK_URI")},
		"verify_token":  {os.Getenv("STRAVA_VERIFY_TOKEN")},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://www.strava.com/api/v3/push_subscriptions", strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("creating subscribe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("subscribing to strava webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return true, nil
	}

	return true, nil
}

// func Unsubscribe() {
// }
