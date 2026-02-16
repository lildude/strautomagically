// Package callback implements the callback handler for the Strava webhook subscription.
package callback

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
)

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	challenge, ok := q["hub.challenge"]
	if !ok {
		http.Error(w, "missing query param: hub.challenge", http.StatusBadRequest)
		return
	}
	verify, ok := q["hub.verify_token"]
	if !ok {
		http.Error(w, "missing query param: hub.verify_token", http.StatusBadRequest)
		return
	}
	if verify[0] != os.Getenv("STRAVA_VERIFY_TOKEN") { // TODO: generate this
		http.Error(w, "verify token mismatch", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge[0]}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("encoding callback response", "error", err)
		return
	}
}
