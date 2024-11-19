package main

import (
	"context"
	"log"
	"net/http"
	"time"
	"trello-project/microservices/tasks-service/handlers"
	"trello-project/microservices/tasks-service/services"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
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

	tasksCollection := tasksClient.Database("mongo-tasks").Collection("tasks")
	projectsCollection := projectsClient.Database("mongo-projects").Collection("projects")

	taskService := services.NewTaskService(tasksCollection, projectsCollection)
	taskHandler := handlers.NewTaskHandler(taskService)

	// // Kreiranje Gorilla Mux routera
	// router := mux.NewRouter()

	// // Definisanje ruta
	// router.HandleFunc("/tasks", taskHandler.CreateTask).Methods("POST")
	// router.HandleFunc("/tasks", taskHandler.GetAllTasks).Methods("GET")

	// // Nove rute za dodavanje i dohvatanje članova zadatka
	// router.HandleFunc("/tasks/{taskID}/project/{projectID}/available-members", taskHandler.GetAvailableMembersForTask).Methods("GET")
	// router.HandleFunc("/tasks/{taskID}/add-members", taskHandler.AddMembersToTask).Methods("POST")
	// router.HandleFunc("/tasks/{taskId}/members", taskHandler.GetMembersForTaskHandler).Methods("GET")

	// // Omogućavanje CORS-a
	// corsRouter := enableCORS(router)

	// // Pokretanje servera
	// log.Println("Server pokrenut na http://localhost:8000")
	// if err := http.ListenAndServe(":8000", corsRouter); err != nil {
	// 	log.Fatal("Greška pri pokretanju servera:", err)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			taskHandler.CreateTask(w, r)
		case http.MethodGet:
			taskHandler.GetAllTasks(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})

	log.Println("Server running on http://localhost:8002")
	if err := http.ListenAndServe(":8002", enableCORS(mux)); err != nil {
		log.Fatal(err)

	}
}
