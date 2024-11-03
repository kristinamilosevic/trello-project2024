package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
	"trello-project/microservices/projects-service/handler"
	"trello-project/microservices/projects-service/service"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Povezivanje sa MongoDB-om
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}

	// Provera konekcije
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB connection error:", err)
	}

	// Inicijalizacija kolekcije u bazi projects
	projectCollection := client.Database("projects").Collection("projects")
	usersCollection := client.Database("users").Collection("users")

	// Kreiranje servisa i handler-a
	projectService := service.NewProjectService(projectCollection, usersCollection)
	projectHandler := handler.NewProjectHandler(projectService)

	// Postavljanje ruta
	r := mux.NewRouter()
	r.HandleFunc("/projects/{id}/members", projectHandler.AddMemberToProjectHandler).Methods("POST")
	r.HandleFunc("/projects/{id}/members", projectHandler.GetProjectMembersHandler).Methods("GET")
	r.HandleFunc("/users", projectHandler.GetAllUsersHandler).Methods("GET")

	// Primeni CORS middleware
	corsRouter := enableCORS(r)

	// Pokretanje servera
	fmt.Println("Server pokrenut na http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", corsRouter))

}
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200") // Dozvoljeno poreklo
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Ako je ovo OPTIONS zahtev, odgovori bez prosleđivanja sledećem handler-u
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
