package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
)

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	challenge, ok := q["hub.challenge"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing query param: hub.challenge")) //nolint:errcheck
		return
	}
	verify, ok := q["hub.verify_token"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("missing query param: hub.verify_token")) //nolint:errcheck
		return
	}
	if strings.Join(verify, "") != os.Getenv("STRAVA_VERIFY_TOKEN") { // TODO: generate this
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("verify token mismatch")) //nolint:errcheck
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"hub.challenge": challenge[0]}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
}
