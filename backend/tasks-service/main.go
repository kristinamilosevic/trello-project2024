package main

import (
	"context"
	"log"
	"net/http"
	"trello-project/microservices/tasks-service/handlers" // Prilagodite putanju
	"trello-project/microservices/tasks-service/services" // Prilagodite putanju

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200") // Angular aplikacija radi na portu 4200
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Kreirajte servis i handler
	taskService := services.NewTaskService(client)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Kreiramo novi multiplexer za rute
	mux := http.NewServeMux()

	// Definišemo POST zahtev za kreiranje taska
	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			taskHandler.CreateTask(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Pokrenemo server sa omogućenim CORS-om
	log.Println("Server pokrenut na http://localhost:8000")
	if err := http.ListenAndServe(":8000", enableCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
