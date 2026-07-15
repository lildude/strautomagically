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

// newStateCookie returns an http.Cookie for the OAuth state with standard security attributes.
// The Secure flag is set only when the request arrived over HTTPS.
func newStateCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     oauthStateCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
}

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
			http.SetCookie(w, newStateCookie(r, oauthState, 600))
			u := strava.OauthConfig.AuthCodeURL(oauthState)
			slog.Info("redirecting to strava auth", "state_len", len(oauthState))
			http.Redirect(w, r, u, http.StatusFound)
		} else {
			http.Redirect(w, r, "/start", http.StatusFound)
		}
		return
	}

	// Validate the returned state against the value stored in the cookie.
	// r.Cookie only ever returns nil or http.ErrNoCookie.
	stateCookie, err := r.Cookie(oauthStateCookie)
	if errors.Is(err, http.ErrNoCookie) {
		slog.Warn("oauth state cookie missing")
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	if stateCookie.Value == "" {
		slog.Warn("oauth state cookie empty")
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(stateCookie.Value), []byte(state)) != 1 {
		slog.Warn("oauth state mismatch")
		http.Error(w, "state invalid", http.StatusBadRequest)
		return
	}
	// Clear the state cookie immediately after successful validation to prevent reuse.
	http.SetCookie(w, newStateCookie(r, "", -1))
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
