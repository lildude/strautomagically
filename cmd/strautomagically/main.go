package main

import (
	"net/http"
	"os"

	// Autoloads .env file to supply environment variables
	_ "github.com/joho/godotenv/autoload"

	"github.com/lildude/strautomagically/internal/handlers/auth"
	"github.com/lildude/strautomagically/internal/handlers/callback"
	"github.com/lildude/strautomagically/internal/handlers/update"
	"github.com/lildude/strautomagically/internal/logger"
)

var Version = "dev"

func main() {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	http.HandleFunc("/start", indexHandler)
	http.HandleFunc("/auth", auth.AuthHandler)
	http.HandleFunc("/webhook", webhookHandler)
	log := logger.NewLogger()

	log.Info("Starting server on port", port)
	log.Fatal(http.ListenAndServe(port, nil)) //#nosec: G114
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.NewLogger()
	w.Header().Set("Strautomagically-Version", Version)
	if _, err := w.Write([]byte("Strautomagically")); err != nil {
		log.Error(err)
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
