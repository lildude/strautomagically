package main

import (
	"log"
	"net/http"
	"os"

	// Autoloads .env file to supply environment variables
	_ "github.com/joho/godotenv/autoload"

	"github.com/lildude/strautomagically/internal/handlers/auth"
	"github.com/lildude/strautomagically/internal/handlers/callback"
	"github.com/lildude/strautomagically/internal/handlers/update"
)

func main() {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	http.HandleFunc("/start", indexHandler)
	http.HandleFunc("/auth", auth.AuthHandler)
	http.HandleFunc("/webhook", webhookHandler)
	log.Println("Starting server on port", port)
	log.Fatal(http.ListenAndServe(port, nil)) //#nosec: G114
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Strautomagically")); err != nil {
		log.Println(err)
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		callback.CallbackHandler(w, r)
	}
	if r.Method == "POST" {
		update.UpdateHandler(w, r)
	}
}
