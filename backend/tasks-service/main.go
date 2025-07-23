package main

import (
	"context"
	"fmt"
	"os"

	// Ostavljamo ga zasad, ali nećemo ga koristiti za logovanje aplikacije
	"net/http"
	"time"
	"trello-project/microservices/tasks-service/handlers"
	"trello-project/microservices/tasks-service/logging" // Vaš prilagođeni logger
	"trello-project/microservices/tasks-service/services"

	http_client "trello-project/backend/utils"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Role, Manager-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
func main() {
	logging.InitLogger() // Inicijalizacija logovanja

	logging.Logger.Info("Event ID: SERVICE_START, Description: Starting Tasks Service...")
	err := godotenv.Load(".env")
	if err != nil {
		logging.Logger.Fatalf("Event ID: ENV_LOAD_ERROR, Description: Error loading .env file: %v", err)
	}

	mongoURI := os.Getenv("MONGO_URI")
	mongoDBName := os.Getenv("MONGO_DB_NAME")
	mongoCollectionName := os.Getenv("MONGO_COLLECTION")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tasksClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logging.Logger.Fatalf("Event ID: DB_CONNECTION_FAILED, Description: Database connection for MongoDB failed: %v", err)
	}
	defer tasksClient.Disconnect(ctx)

	if err := tasksClient.Ping(ctx, nil); err != nil {
		logging.Logger.Fatalf("Event ID: DB_PING_FAILED, Description: MongoDB connection ping error: %v", err)
	}
	logging.Logger.Infof("Event ID: DB_CONNECTED, Description: Successfully connected to MongoDB at %s.", mongoURI)

	tasksCollection := tasksClient.Database(mongoDBName).Collection(mongoCollectionName)
	logging.Logger.Infof("Event ID: DB_COLLECTION_SET, Description: Using MongoDB collection: %s/%s", mongoDBName, mongoCollectionName)
	httpClient := http_client.NewHTTPClient()

	projectsBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ProjectsServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Infof("Event ID: CIRCUIT_BREAKER_STATE_CHANGE, Description: Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})
	notificationsbreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "notifications-cb",
		MaxRequests: 1,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Infof("Event ID: CIRCUIT_BREAKER_STATE_CHANGE, Description: Circuit Breaker '%s' state changed from %s to %s", name, from.String(), to.String())
		},
	})

	taskService := services.NewTaskService(tasksCollection, httpClient, projectsBreaker, notificationsbreaker)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Kreiranje mux routera
	r := mux.NewRouter()

	// Definisanje rute sa parametrima za zadatke i članove
	r.HandleFunc("/api/tasks/{taskID}/project/{projectID}/available-members", taskHandler.GetAvailableMembersForTask).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks/{taskID}/add-members", taskHandler.AddMembersToTask).Methods(http.MethodPost)
	r.HandleFunc("/api/tasks/{taskID}/members", taskHandler.GetMembersForTaskHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks/{taskID}/members/{memberID}", taskHandler.RemoveMemberFromTaskHandler).Methods(http.MethodDelete)
	r.HandleFunc("/api/tasks/all", taskHandler.GetAllTasks).Methods("GET")                         // Prikaz svih zadataka
	r.HandleFunc("/api/tasks/create", taskHandler.CreateTask).Methods("POST")                      // Kreiranje novog zadatka
	r.HandleFunc("/api/tasks/project/{projectId}", taskHandler.GetTasksByProjectID).Methods("GET") // Zadatke po ID-u projekta
	r.HandleFunc("/api/tasks/status", taskHandler.ChangeTaskStatus).Methods("POST")
	r.HandleFunc("/api/tasks/project/{projectId}", taskHandler.DeleteTasksByProjectHandler).Methods(http.MethodDelete)
	r.HandleFunc("/api/tasks/has-active", taskHandler.HasActiveTasksHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/tasks/project/{projectId}/has-unfinished", taskHandler.HasUnfinishedTasksHandler).Methods("GET")
	r.HandleFunc("/api/tasks/remove-user/by-username/{username}", taskHandler.RemoveUserFromAllTasksByUsername).Methods("PATCH")

	corsRouter := enableCORS(r)

	// Svi ostali taskovi .
	r.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			taskHandler.CreateTask(w, r)
		case http.MethodGet:
			taskHandler.GetAllTasks(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		}
	})
	// Pokretanje servera
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		logging.Logger.Fatalf("Event ID: CONFIG_ERROR, Description: SERVER_PORT is not set in the environment variables.")
	}

	serverAddress := fmt.Sprintf(":%s", serverPort)
	logging.Logger.Infof("Event ID: SERVER_START_INFO, Description: Server running on http://localhost%s", serverAddress)

	if err := http.ListenAndServe(serverAddress, corsRouter); err != nil {
		logging.Logger.Fatalf("Event ID: SERVER_FATAL_ERROR, Description: Server failed to start: %v", err)
	}

}
