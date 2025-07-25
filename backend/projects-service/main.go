package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"trello-project/microservices/projects-service/handlers"
	"trello-project/microservices/projects-service/logging"
	"trello-project/microservices/projects-service/services"

	http_client "trello-project/backend/utils"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func createProjectNameIndex(collection *mongo.Collection) error {
	indexModel := mongo.IndexModel{
		Keys:    bson.M{"name": 1},
		Options: options.Index().SetUnique(true),
	}
	_, err := collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		return fmt.Errorf("failed to create unique index on project name: %v", err)
	}
	logging.Logger.Info("Unique index on project name created successfully")
	return nil
}

func main() {
	logging.InitLogger() // Inicijalizacija logovanja

	logging.Logger.Info("Starting Projects Service...")
	if err := godotenv.Load(".env"); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	mongoURI, mongoDBName, mongoCollectionName := os.Getenv("MONGO_URI"), os.Getenv("MONGO_DB_NAME"), os.Getenv("MONGO_COLLECTION")
	if mongoURI == "" || mongoDBName == "" || mongoCollectionName == "" {
		logging.Logger.Fatal("Missing required environment variables for MongoDB")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	projectsClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		logging.Logger.Fatalf("Database connection for mongo-projects failed: %v", err)
	}
	defer projectsClient.Disconnect(ctx)

	if err := projectsClient.Ping(ctx, nil); err != nil {
		logging.Logger.Fatalf("MongoDB connection error for mongo-projects: %v", err)
	}

	projectsDB := projectsClient.Database(mongoDBName)

	httpClient := http_client.NewHTTPClient()

	tasksBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "TasksServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Infof("Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})

	usersBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "UsersServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Infof("Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})
	notificationsBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "NotificationsServiceCB",
		MaxRequests: 1,
		Timeout:     5 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Infof("Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})

	projectService := services.NewProjectService(
		projectsDB.Collection(mongoCollectionName),
		httpClient,
		tasksBreaker,
		usersBreaker,
		notificationsBreaker,
	)

	//projectService := services.NewProjectService(projectsDB.Collection(mongoCollectionName), httpClient)

	if err := createProjectNameIndex(projectsDB.Collection(mongoCollectionName)); err != nil {
		logging.Logger.Fatal(err)
	}

	projectHandler := handlers.NewProjectHandler(projectService)

	r := mux.NewRouter()
	r.HandleFunc("/api/projects/{projectId}/members/all", projectHandler.GetProjectMembersHandler).Methods("GET")
	r.HandleFunc("/api/projects/remove/{projectId}/members/{memberId}/remove", projectHandler.RemoveMemberFromProjectHandler).Methods("DELETE")
	r.HandleFunc("/api/projects/add", projectHandler.CreateProject).Methods("POST")
	r.HandleFunc("/api/projects/{id}/members", projectHandler.AddMemberToProjectHandler).Methods("POST")
	r.HandleFunc("/api/projects/users", projectHandler.GetAllUsersHandler).Methods("GET")
	r.HandleFunc("/api/projects/all", projectHandler.ListProjectsHandler).Methods("GET")
	r.HandleFunc("/api/projects/username/{username}", handlers.GetProjectsByUsername(projectService)).Methods("GET", "OPTIONS")
	r.HandleFunc("/api/projects/{id}", projectHandler.GetProjectByIDHandler).Methods("GET")
	r.HandleFunc("/api/projects/{id}/tasks", projectHandler.DisplayTasksForProjectHandler).Methods("GET")
	r.HandleFunc("/api/projects/{projectId}", projectHandler.RemoveProjectHandler).Methods(http.MethodDelete)
	r.HandleFunc("/api/projects/members", projectHandler.GetAllMembersHandler)
	r.HandleFunc("/api/projects/{projectId}/add-task", projectHandler.AddTaskToProjectHandler).Methods("POST")
	r.HandleFunc("/api/projects/user-projects/{username}", handlers.GetProjectsByUsername(projectService)).Methods("GET")
	r.HandleFunc("/api/projects/remove-user/{userID}", projectHandler.RemoveUserFromProjectsHandler).Methods("PATCH")

	corsRouter := enableCORS(r)

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		logging.Logger.Fatal("SERVER_PORT is not set in the environment variables")
	}

	serverAddress := fmt.Sprintf(":%s", serverPort)

	logging.Logger.Infof("Projects service server running on %s", serverAddress)
	logging.Logger.Fatal(http.ListenAndServe(serverAddress, corsRouter))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Role, Manager-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
