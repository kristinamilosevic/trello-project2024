package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"trello-project/microservices/projects-service/handlers"
	"trello-project/microservices/projects-service/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// CORS middleware function
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	// Povezivanje sa MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer client.Disconnect(context.TODO())

	projectsDB := client.Database("projects_db")
	tasksDB := client.Database("tasks_db")

	projectService := &services.ProjectService{
		ProjectsCollection: projectsDB.Collection("project"),
		TasksCollection:    tasksDB.Collection("tasks"),
	}
	projectHandler := handlers.NewProjectHandler(projectService)

	r := mux.NewRouter()
	r.HandleFunc("/projects/{projectId}/members", projectHandler.GetProjectMembersHandler).Methods("GET")
	r.HandleFunc("/projects/{projectId}/members/{memberId}/remove", projectHandler.RemoveMemberFromProjectHandler).Methods("DELETE") // Ruta za uklanjanje člana

	corsRouter := enableCORS(r)

	// Pokretanje servera
	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", corsRouter))
}
