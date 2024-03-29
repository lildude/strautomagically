// Package auth implements the authentication handler.
package auth

import (
	"log"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
	"golang.org/x/oauth2"
)

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println("[ERROR] unable to parse form:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STATE_TOKEN")
	che, err := cache.NewRedisCache(os.Getenv("REDIS_URL")) //nolint:contextcheck // TODO: pass context rather then generate in the package.
	if err != nil {
		log.Println("[ERROR] unable to create redis cache:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authToken := &oauth2.Token{}
	che.GetJSON("strava_auth_token", &authToken) //nolint:gosec // We don't care if this fails

	if state == "" {
		if authToken.AccessToken == "" {
			u := strava.OauthConfig.AuthCodeURL(stateToken)
			log.Println("[INFO] redirecting to", u)
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/start", http.StatusFound)
		}
		return
	}

	if state != stateToken {
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	code := r.Form.Get("code")
	if code == "" {
		http.Error(w, "code not found", http.StatusBadRequest)
		return
	}
	token, err := strava.OauthConfig.Exchange(r.Context(), code)
	if err != nil {
		log.Println("[ERROR]", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	athlete, ok := token.Extra("athlete").(map[string]interface{})
	if !ok {
		log.Println("[ERROR] unable to get athete info", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = che.SetJSON("strava_auth_token", token)
	if err != nil {
		log.Println("[ERROR]", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Println("[INFO] successfully authenticated:", athlete["username"])

	// Subscribe to the activity stream - should this be here?
	ok, err = Subscribe()
	if !ok {
		log.Println("[ERROR] failed to subscribe to strava webhook:", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	log.Println("[INFO] successfully subscribed to Strava activity feed")

	http.Redirect(w, r, "/start", http.StatusFound)
}
