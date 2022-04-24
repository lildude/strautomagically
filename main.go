package main

import (
	"log"
	"net/http"
	"os"

	// Autoloads .env file to supply environment variables
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/auth", authHandler)
	http.HandleFunc("/callback", callbackHandler)
	http.HandleFunc("/update", updateHandler)
	log.Println("Starting server on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Strautomagically")); err != nil {
		log.Println(err)
	}
}
