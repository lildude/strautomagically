package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
	"golang.org/x/oauth2"
)

// Transport can be overridden for the purpose of testing.
var Transport http.RoundTripper = &http.Transport{}
var ctx = context.Background()

func newStravaClient() *strava.APIClient {
	authToken, err := getToken("strava_auth_token")
	if err != nil {
		log.Printf("Unable to get token: %s", err)
		return nil
	}
	// The Oauth2 library handles refreshing the token if it's expired.
	tokenSource := oauthConfig.TokenSource(context.Background(), authToken)
	cfg := strava.NewConfiguration()
	cfg.HTTPClient = &http.Client{Transport: Transport}
	ctx = context.WithValue(ctx, strava.ContextOAuth2, tokenSource)

	// Update our saved token
	newToken, err := tokenSource.Token()
	if err != nil {
		panic(err)
	}
	if newToken.AccessToken != authToken.AccessToken {
		err = setToken("strava_auth_token", newToken)
		if err != nil {
			log.Printf("Unable to store token: %s", err)
			return nil
		}
		log.Println("Updated token")
	}

	return strava.NewAPIClient(cfg)
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
