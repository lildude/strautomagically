package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"golang.org/x/oauth2"
)

const challengeKey = "hub.challenge"

type callbackResponse struct {
	Challenge string `json:"hub.challenge"`
}

var oauthConfig = &oauth2.Config{
	ClientID:     os.Getenv("STRAVA_CLIENT_ID"),
	ClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
	Endpoint: oauth2.Endpoint{
		AuthURL:  "https://www.strava.com/oauth/authorize",
		TokenURL: "https://www.strava.com/oauth/token",
	},
	RedirectURL: os.Getenv("STRAVA_REDIRECT_URI"),
	Scopes:      []string{"activity:write,activity:read_all"},
}

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	// If we already have an auth token we know this is a webhook subscription request or oauth callback
	cache := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	authToken := &oauth2.Token{}
	at, err := cache.Get("strava_auth_token")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if at != "" {
		err = json.Unmarshal([]byte(fmt.Sprint(at)), &authToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	stateToken := os.Getenv("STATE_TOKEN")
	err = r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	state := r.Form.Get("state")
	if state != "" {
		if state != stateToken {
			http.Error(w, "State invalid", http.StatusBadRequest)
			return
		}
		code := r.Form.Get("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
			return
		}
		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Pretty print the token for debugging
		e := json.NewEncoder(w)
		e.SetIndent("", "  ")
		e.Encode(*token)

		t, err := json.Marshal(&token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = cache.Set("strava_auth_token", string(t))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		return
	} else if authToken.AccessToken == "" {
		u := oauthConfig.AuthCodeURL(stateToken)
		fmt.Println("Redirecting to", u)
		http.Redirect(w, r, u, http.StatusFound)
	}

	challenge := r.URL.Query().Get(challengeKey)
	if challenge == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(fmt.Sprintf("missing query param: %s\n", challengeKey))); err != nil {
			log.Println(err)
		}
		return
	}
	resp, err := json.Marshal(callbackResponse{
		Challenge: challenge,
	})
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(fmt.Sprintf("%s\n", resp))); err != nil {
		log.Println(err)
	}
}
