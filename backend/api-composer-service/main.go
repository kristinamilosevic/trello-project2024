package main

import (
	"log"
	"net/http"
	"trello-project/microservices/api-composer-service/handlers"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/api/graph/{projectId}", handlers.GetGraphHandler).Methods("GET")

	log.Println("API Composer Service running on port 8006...")
	if err := http.ListenAndServe(":8006", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
