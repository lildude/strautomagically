package auth

import (
	"context"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
	"golang.org/x/oauth2"
)

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Errorf("unable to parse form: %s\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STATE_TOKEN")
	che, err := cache.NewRedisCache(os.Getenv("REDIS_URL"))
	if err != nil {
		log.Errorf("unable to create redis cache: %s\n", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authToken := &oauth2.Token{}
	che.GetJSON("strava_auth_token", &authToken) //nolint:errcheck

	if state == "" {
		if authToken.AccessToken == "" {
			u := strava.OauthConfig.AuthCodeURL(stateToken)
			log.Debugln("redirecting to", u)
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/start", http.StatusFound)
		}
	} else {
		if state != stateToken {
			http.Error(w, "state invalid", http.StatusBadRequest)
			return
		}
		code := r.Form.Get("code")
		if code == "" {
			http.Error(w, "code not found", http.StatusBadRequest)
			return
		}
		token, err := strava.OauthConfig.Exchange(context.Background(), code)
		if err != nil {
			log.Errorln(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		athlete, ok := token.Extra("athlete").(map[string]interface{})
		if !ok {
			log.Errorln("unable to get athete info", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		err = che.SetJSON("strava_auth_token", token)
		if err != nil {
			log.Errorln(err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		log.Debugf("successfully authenticated: %s", athlete["username"])

		// Subscribe to the activity stream - should this be here?
		ok, err = Subscribe()
		if !ok {
			log.Errorln("failed to subscribe to strava webhook:", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		log.Infoln("successfully subscribed to Strava activity feed")

		http.Redirect(w, r, "/start", http.StatusFound)
	}
}
