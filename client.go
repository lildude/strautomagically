package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strava-swagger"
	"golang.org/x/oauth2"
)

// Transport can be overridden for the purpose of testing.
var Transport http.RoundTripper = &http.Transport{}
var ctx = context.Background()

func newStravaClient() (*strava.APIClient, error) {
	cache, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Printf("unable to create redis cache: %s", err)
		return nil, err
	}

	authToken := &oauth2.Token{}
	err = cache.GetJSON("strava_auth_token", &authToken)

	if err != nil {
		log.Printf("unable to get token from redis: %s", err)
		return nil, err
	}
	// The Oauth2 library handles refreshing the token if it's expired.
	tokenSource := oauthConfig.TokenSource(context.Background(), authToken)
	cfg := strava.NewConfiguration()
	cfg.HTTPClient = &http.Client{Transport: Transport}
	ctx = context.WithValue(ctx, strava.ContextOAuth2, tokenSource)

	// Update our saved token
	newToken, err := tokenSource.Token()
	if err != nil {
		log.Printf("unable to refresh token: %s", err)
		return nil, err
	}
	if newToken.AccessToken != authToken.AccessToken {
		err = cache.SetJSON("strava_auth_token", newToken)
		if err != nil {
			log.Printf("unable to store token: %s", err)
			return nil, err
		}
		log.Println("updated token")
	}

	return strava.NewAPIClient(cfg), nil
}
