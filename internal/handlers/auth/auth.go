// Package auth implements the authentication handler.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/lildude/strautomagically/internal/cache"
	"github.com/lildude/strautomagically/internal/strava"
	"golang.org/x/oauth2"
)

const oauthStateCookie = "oauth_state"

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		slog.Error("unable to parse form", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	state := r.Form.Get("state")
	che, err := cache.NewRedisCache(r.Context(), os.Getenv("REDIS_URL"))
	if err != nil {
		slog.Error("unable to create redis cache", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	authToken := &oauth2.Token{}
	if err := che.GetJSON(r.Context(), "strava_auth_token", &authToken); err != nil {
		slog.Warn("unable to get cached auth token", "error", err)
	}

	if state == "" {
		if authToken.AccessToken == "" {
			// Generate a cryptographically random per-request state to prevent CSRF attacks.
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				slog.Error("failed to generate OAuth state", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			oauthState := hex.EncodeToString(b)
			http.SetCookie(w, &http.Cookie{
				Name:     oauthStateCookie,
				Value:    oauthState,
				Path:     "/",
				HttpOnly: true,
				Secure:   true,
				SameSite: http.SameSiteLaxMode,
				MaxAge:   600, // 10 minutes
			})
			u := strava.OauthConfig.AuthCodeURL(oauthState)
			slog.Info("redirecting to strava auth")
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/start", http.StatusFound)
		}
		return
	}

	// Validate the returned state against the value stored in the cookie.
	stateCookie, err := r.Cookie(oauthStateCookie)
	if errors.Is(err, http.ErrNoCookie) {
		slog.Warn("oauth state cookie missing")
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("unexpected error reading oauth state cookie", "error", err)
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	if stateCookie.Value == "" || subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		slog.Warn("oauth state mismatch")
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	// Clear the state cookie immediately after successful validation to prevent reuse.
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
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
