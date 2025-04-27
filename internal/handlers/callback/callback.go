// Package callback implements the callback handler for the Strava webhook subscription.
package callback

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	challenge, ok := q["hub.challenge"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing query param: hub.challenge")) //nolint:gosec // We don't care if this fails
		return
	}
	verify, ok := q["hub.verify_token"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing query param: hub.verify_token")) //nolint:gosec // We don't care if this fails
		return
	}
	if strings.Join(verify, "") != os.Getenv("STRAVA_VERIFY_TOKEN") { // TODO: generate this
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("verify token mismatch")) //nolint:gosec // We don't care if this fails
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge[0]}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logrus.WithError(err).Error("Failed to encode hub.challenge response")
		return
	}
}
