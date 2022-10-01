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
	if os.Getenv("DYNO") != "" {
		log.SetFlags(0)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/auth", auth.AuthHandler)
	// http.HandleFunc("/callback", callback.CallbackHandler)
	// http.HandleFunc("/update", update.UpdateHandler)
	http.HandleFunc("/webhook", webhookHandler)
	log.Println("Starting server on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
