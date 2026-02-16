package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	// Autoloads .env file to supply environment variables.
	_ "github.com/joho/godotenv/autoload"

	"github.com/lildude/strautomagically/internal/handlers/auth"
	"github.com/lildude/strautomagically/internal/handlers/callback"
	"github.com/lildude/strautomagically/internal/handlers/update"
)

var Version = "dev"

func main() {
	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/start", indexHandler)
	mux.HandleFunc("/auth", auth.AuthHandler)
	mux.HandleFunc("/webhook", webhookHandler)

	srv := &http.Server{
		Addr:              port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	slog.Info("starting server", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Strautomagically-Version", Version)
	if _, err := w.Write([]byte("Strautomagically")); err != nil {
		slog.Error("write failed", "error", err)
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		callback.CallbackHandler(w, r)
	case http.MethodPost:
		update.UpdateHandler(w, r)
	}
}
