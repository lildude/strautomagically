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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("Unable to parse form: %s", err)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STATE_TOKEN")
	authToken, err := cache.GetToken()
	if err != nil {
		log.Printf("Unable to get token: %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	if state == "" {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("Unable to connect to redis: %s", err)
			return
		}

		if authToken.AccessToken == "" {
			u := oauthConfig.AuthCodeURL(stateToken)
			log.Println("Redirecting to", u)
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/", http.StatusFound)
		}
	} else {
		if state != stateToken {
			http.Error(w, "State invalid", http.StatusBadRequest)
		}
		code := r.Form.Get("code")
		if code == "" {
			http.Error(w, "Code not found", http.StatusBadRequest)
		}
		token, err := oauthConfig.Exchange(context.Background(), code)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		athlete, ok := token.Extra("athlete").(map[string]interface{})
		if !ok {
			log.Println("unable to get athete info", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		err = cache.SetToken(token)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		log.Printf("Successfully authenticated: %s", athlete["username"])
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
