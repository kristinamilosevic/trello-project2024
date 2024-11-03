package main

import (
	"context"
	"log"
	"net/http"
	"trello-project/microservices/projects-service/handlers" // Prilagodite putanju ako je potrebno
	"trello-project/microservices/projects-service/services" // Prilagodite putanju ako je potrebno

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// enableCORS omogućava CORS za Angular aplikaciju koja radi na portu 4200
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Manager-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Konektovanje na MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// Kreirajte ProjectService i ProjectHandler
	projectService := services.NewProjectService(client)
	projectHandler := handlers.NewProjectHandler(projectService)

	// Kreiramo novi multiplexer za rute
	mux := http.NewServeMux()

	// Definišemo POST zahtev za kreiranje projekta
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			projectHandler.CreateProject(w, r)
		} else {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	// Pokrećemo server sa omogućenim CORS-om
	log.Println("Projects service server pokrenut na http://localhost:8001")
	if err := http.ListenAndServe(":8001", enableCORS(mux)); err != nil {
		log.Fatal(err)
	}
}
