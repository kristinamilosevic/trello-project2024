package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"trello-project/microservices/projects-service/handlers"
	"trello-project/microservices/projects-service/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
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
	fmt.Println("Unique index on project name created successfully")
	return nil
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		log.Fatal("JWT_SECRET is not set in the environment variables")
	}

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		log.Fatal("MONGO_URI is not set in the environment variables")
	}

	mongoDBName := os.Getenv("MONGO_DB_NAME")
	if mongoDBName == "" {
		log.Fatal("MONGO_DB_NAME is not set in the environment variables")
	}

	mongoCollectionName := os.Getenv("MONGO_COLLECTION")
	if mongoCollectionName == "" {
		log.Fatal("MONGO_COLLECTION is not set in the environment variables")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	projectsClient, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal("Database connection for mongo-projects failed:", err)
	}
	defer projectsClient.Disconnect(context.TODO())

	if err := projectsClient.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB connection error for mongo-projects:", err)
	}

	projectsDB := projectsClient.Database(mongoDBName)

	projectService := &services.ProjectService{
		ProjectsCollection: projectsDB.Collection(mongoCollectionName),
	}

	if err := createProjectNameIndex(projectsDB.Collection(mongoCollectionName)); err != nil {
		log.Fatal(err)
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

	corsRouter := enableCORS(r)

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		log.Fatal("SERVER_PORT is not set in the environment variables")
	}

	serverAddress := fmt.Sprintf(":%s", serverPort)

	fmt.Println("Projects service server running on", serverAddress)
	log.Fatal(http.ListenAndServe(serverAddress, corsRouter))

}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("CORS_ORIGIN"))
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Role, Manager-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
