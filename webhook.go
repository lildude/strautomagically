package main

import "net/http"

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		callbackHandler(w, r)
	}
	if r.Method == "POST" {
		updateHandler(w, r)
	}
}
