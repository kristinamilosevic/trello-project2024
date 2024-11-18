package main

import (
	"context"
	"log"
	"net/http"
	"trello-project/microservices/tasks-service/handlers" // Prilagodite putanju
	"trello-project/microservices/tasks-service/services" // Prilagodite putanju

	"github.com/gorilla/mux"
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
	clientOptions := options.Client().ApplyURI("mongodb://mongo:27017")
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	taskService := services.NewTaskService(client)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Kreiranje Gorilla Mux routera
	router := mux.NewRouter()

	// Definisanje ruta
	router.HandleFunc("/tasks", taskHandler.CreateTask).Methods("POST")
	router.HandleFunc("/tasks", taskHandler.GetAllTasks).Methods("GET")

	// Nove rute za dodavanje i dohvatanje članova zadatka
	router.HandleFunc("/tasks/{taskID}/project/{projectID}/available-members", taskHandler.GetAvailableMembersForTask).Methods("GET")
	router.HandleFunc("/tasks/{taskID}/add-members", taskHandler.AddMembersToTask).Methods("POST")
	router.HandleFunc("/tasks/{taskId}/members", taskHandler.GetMembersForTaskHandler).Methods("GET")

	// Omogućavanje CORS-a
	corsRouter := enableCORS(router)

	// Pokretanje servera
	log.Println("Server pokrenut na http://localhost:8000")
	if err := http.ListenAndServe(":8000", corsRouter); err != nil {
		log.Fatal("Greška pri pokretanju servera:", err)
	}
}
