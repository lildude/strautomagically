package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env"); if err != nil {
		log.Fatal("Error loading .env file")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/", Handler)
	log.Println("Starting server on port", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		CallbackHandler(w, r)
	case http.MethodPost:
		UpdateHandler(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		if _, err := w.Write([]byte(fmt.Sprintf("unsupported method: %s\n", r.Method))); err != nil {
			log.Println(err)
		}
	}
}
