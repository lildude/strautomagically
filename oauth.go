package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"golang.org/x/oauth2"
)

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

func authHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Printf("unable to parse form: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STATE_TOKEN")
	cache, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Printf("unable to create redis cache: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	authToken := &oauth2.Token{}
	err = cache.GetJSON("strava_auth_token", &authToken)
	if err != nil {
		log.Printf("unable to get token: %s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if state == "" {
		if authToken.AccessToken == "" {
			u := oauthConfig.AuthCodeURL(stateToken)
			log.Println("redirecting to", u)
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
	} else {
		if state != stateToken {
			http.Error(w, "state invalid", http.StatusBadRequest)
		}
		code := r.Form.Get("code")
		if code == "" {
			http.Error(w, "code not found", http.StatusBadRequest)
		}
		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		athlete, ok := token.Extra("athlete").(map[string]interface{})
		if !ok {
			log.Println("unable to get athete info", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}

		err = cache.SetJSON("strava_auth_token", token)
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
		log.Printf("successfully authenticated: %s", athlete["username"])
		http.Redirect(w, r, "/", http.StatusFound)

		// Subscribe to the activity stream - should this be here?
		err = Subscribe()
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}
	}
}
