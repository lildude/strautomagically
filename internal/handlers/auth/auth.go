// Package auth implements the authentication handler.
package auth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
)

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		slog.Error("unable to parse form", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STATE_TOKEN")

	db, err := database.InitDB()
	if err != nil {
		slog.Error("unable to connect to database", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// No state means this is the start of the auth flow: redirect to Strava to
	// authenticate, unless we already have a token stored.
	if state == "" {
		var athlete model.Athlete
		db.First(&athlete)
		if athlete.StravaAuthToken != "" {
			http.Redirect(w, r, "/start", http.StatusFound)
		} else {
			u := strava.OauthConfig.AuthCodeURL(stateToken)
			slog.Info("redirecting to strava auth", "url", u)
			http.Redirect(w, r, u, http.StatusFound)
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
		slog.Error("token exchange failed", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	athleteInfo, ok := token.Extra("athlete").(map[string]any)
	if !ok {
		slog.Error("unable to get athlete info")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		slog.Error("unable to marshal token", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	athleteID, _ := athleteInfo["id"].(float64)
	athleteName, _ := athleteInfo["username"].(string)

	// Insert or update the athlete in the database
	var athlete model.Athlete
	db.Where(model.Athlete{StravaAthleteID: int64(athleteID)}).FirstOrCreate(&athlete)
	athlete.StravaAthleteName = athleteName
	athlete.StravaAuthToken = string(tokenJSON)
	db.Save(&athlete)

	slog.Info("successfully authenticated", "username", athleteName)

	// Subscribe to the activity stream - should this be here?
	ok, err = Subscribe(r.Context())
	if !ok {
		slog.Error("failed to subscribe to strava webhook", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	slog.Info("successfully subscribed to strava activity feed")

	http.Redirect(w, r, "/start", http.StatusFound)
}
