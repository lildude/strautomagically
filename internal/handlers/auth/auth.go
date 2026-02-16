// Package auth implements the authentication handler.
package auth

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
	"golang.org/x/oauth2"
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
	che, err := cache.NewRedisCache(r.Context(), os.Getenv("REDIS_URL"))
	if err != nil {
		slog.Error("unable to create redis cache", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authToken := &oauth2.Token{}
	che.GetJSON(r.Context(), "strava_auth_token", &authToken) //nolint:gosec // We don't care if this fails

	if state == "" {
		if authToken.AccessToken == "" {
			u := strava.OauthConfig.AuthCodeURL(stateToken)
			slog.Info("redirecting to strava auth", "url", u)
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
		slog.Error("token exchange failed", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	athlete, ok := token.Extra("athlete").(map[string]any)
	if !ok {
		slog.Error("unable to get athlete info")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	err = che.SetJSON(r.Context(), "strava_auth_token", token)
	if err != nil {
		slog.Error("unable to store token", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	slog.Info("successfully authenticated", "username", athlete["username"])

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
