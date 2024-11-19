package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
	"trello-project/microservices/tasks-service/handlers"
	"trello-project/microservices/tasks-service/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// MongoDB konekcija
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasksClient, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongo-tasks:27017"))
	if err != nil {
		log.Fatal("Database connection for mongo-tasks failed:", err)
	}
	defer tasksClient.Disconnect(ctx)

	projectsClient, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongo-projects:27017"))
	if err != nil {
		log.Fatal("Database connection for mongo-projects failed:", err)
	}
	defer projectsClient.Disconnect(ctx)

	if err := tasksClient.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB connection error for mongo-tasks:", err)
	}
	if err := projectsClient.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB connection error for mongo-projects:", err)
	}

	// Kolekcije
	tasksCollection := tasksClient.Database("mongo-tasks").Collection("tasks")
	projectsCollection := projectsClient.Database("mongo-projects").Collection("projects")

	// Servisi i handleri
	taskService := services.NewTaskService(tasksCollection, projectsCollection)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Kreiranje routera
	r := mux.NewRouter()

	// Rute za zadatke
	r.HandleFunc("/api/tasks/all", taskHandler.GetAllTasks).Methods("GET")                         // Prikaz svih zadataka
	r.HandleFunc("/api/tasks/create", taskHandler.CreateTask).Methods("POST")                      // Kreiranje novog zadatka
	r.HandleFunc("/api/tasks/project/{projectId}", taskHandler.GetTasksByProjectID).Methods("GET") // Zadatke po ID-u projekta
	r.HandleFunc("/api/tasks/status", taskHandler.ChangeTaskStatus).Methods("POST")                // Promena statusa zadatka

	// Omogućavanje CORS-a
	corsRouter := enableCORS(r)

	// Pokretanje servera
	fmt.Println("Tasks service server running on http://localhost:8002")
	log.Fatal(http.ListenAndServe(":8002", corsRouter))
}
