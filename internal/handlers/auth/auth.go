// Package auth implements the authentication handler.
package auth

import (
	"net/http"
	"os"

	"github.com/jackc/pgtype"
	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/model"
	"github.com/lildude/strautomagically/internal/strava"
	"github.com/sirupsen/logrus"
)

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		logrus.WithError(err).Error("Unable to parse form")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	stateToken := os.Getenv("STRAVA_STATE_TOKEN")

	db, err := database.InitDB()
	if err != nil {
		logrus.WithError(err).Error("Unable to connect to database")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	var athlete model.Athlete

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
		logrus.WithError(err).Error("Failed to exchange OAuth code")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	athleteInfo, ok := token.Extra("athlete").(map[string]interface{})
	if !ok {
		logrus.Error("Unable to get athlete info", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Insert or update the athlete in the database
	// Check if the athlete already exists
	err = db.Where("strava_athlete_id = ?", int64(athleteInfo["id"].(float64))).First(&athlete).Error
	if err != nil && err.Error() != "record not found" {
		logrus.WithError(err).Error("Unable to find athlete")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err == nil {
		// Athlete exists, update the record
		if err := athlete.StravaAuthToken.Set(token); err != nil {
			logrus.WithError(err).Error("failed to set StravaAuthToken")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		db.Save(&athlete)
		logrus.Infof("Successfully updated athlete: %s", athleteInfo["username"])
	} else {
		// Athlete does not exist, create a new record
		athlete.StravaAuthToken = pgtype.JSONB{}
		if err := athlete.StravaAuthToken.Set(token); err != nil {
			logrus.WithError(err).Error("Failed to set StravaAuthToken")
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		athlete.StravaAthleteID = int64(athleteInfo["id"].(float64))
		athlete.StravaAthleteName = athleteInfo["username"].(string)
		athlete.LastActivityID = 0

		db.Create(&athlete)
		logrus.Infof("Successfully authenticated: %s", athleteInfo["username"])
	}

	// Subscribe to the activity stream - should this be here?
	ok, err = Subscribe()
	if !ok {
		logrus.WithError(err).Error("Failed to subscribe to strava webhook")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	logrus.Info("Successfully subscribed to Strava activity feed")

	http.Redirect(w, r, "/start", http.StatusFound)
}
