package main

import (
	"context"
	"log"
	"net/http"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
)

// Transport can be overridden for the purpose of testing.
var Transport http.RoundTripper = &http.Transport{}
var ctx = context.Background()

func newStravaClient() *strava.APIClient {
	authToken, err := cache.GetToken("strava_auth_token")
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
		err = cache.SetToken("strava_auth_token", newToken)
		if err != nil {
			log.Printf("Unable to store token: %s", err)
			return nil
		}
		log.Println("Updated token")
	}

	return strava.NewAPIClient(cfg)
}
