package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

const challengeKey = "hub.challenge"

type callbackResponse struct {
	Challenge string `json:"hub.challenge"`
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get(ChallengeKey)
	if challenge == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(fmt.Sprintf("missing query param: %s\n", challengeKey))); err != nil {
			log.Println(err)
		}
		return
	}
	resp, err := json.Marshal(callbackResponse{
		Challenge: challenge,
	})

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		if _, err = w.Write([]byte(fmt.Sprintf("%s\n", err))); err != nil {
			log.Println(err)
		}
		return
	}
	w.WriteHeader(http.StatusOK)
	if _, err = w.Write([]byte(fmt.Sprintf("%s\n", resp))); err != nil {
		log.Println(err)
	}

}