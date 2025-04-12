package main

import (
	"log"
	"net/http"
	"os"

	// Autoloads .env file to supply environment variables.
	_ "github.com/joho/godotenv/autoload"

	"github.com/lildude/strautomagically/internal/database"
	"github.com/lildude/strautomagically/internal/handlers/auth"
	"github.com/lildude/strautomagically/internal/handlers/callback"
	"github.com/lildude/strautomagically/internal/handlers/update"
	"github.com/lildude/strautomagically/internal/strava"
)

var Version = "dev"

func main() {
	// Initialize the database
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Pass the database instance to handlers if needed
	_ = db

	port := ":8080"
	if val, ok := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT"); ok {
		port = ":" + val
	}
	http.HandleFunc("/start", indexHandler)
	http.HandleFunc("/auth", auth.AuthHandler)
	http.HandleFunc("/webhook", webhookHandler)

	log.SetFlags(0)
	log.Println("[INFO] Starting server on port", port)
	log.Fatal(http.ListenAndServe(port, nil)) //#nosec: G114
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Strautomagically-Version", Version)
	// If DISABLE_SIGNUP is set, return a 200 OK response
	if os.Getenv("DISABLE_SIGNUP") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	} else {
		stateToken := os.Getenv("STRAVA_ACCESS_TOKEN")
		url := strava.OauthConfig.AuthCodeURL(stateToken)
		if _, err := w.Write([]byte(`<!DOCTYPE html>
			<html>
			<head>
				<title>Strautomagically</title>
			</head>
			<body>
				<h1>Strautomagically</h1>
				<p>
				<a href="` + url + `" style="display: inline-block; padding: 10px 20px; background-color: #fc5200; color: white; text-decoration: none; border-radius: 5px;">Connect</a>
				</p>
			</body>
			</html>`)); err != nil {
			log.Println("[ERROR]", err)
		}
	}
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		callback.CallbackHandler(w, r)
	}
	if r.Method == http.MethodPost {
		update.UpdateHandler(w, r)
	}
}
