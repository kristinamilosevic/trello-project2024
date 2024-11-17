package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"trello-project/microservices/projects-service/handlers"
	"trello-project/microservices/projects-service/services"

	"github.com/gorilla/mux"
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
	// Connect to MongoDB

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://mongo:27017"))
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer client.Disconnect(context.TODO())

	// Provera konekcije
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal("MongoDB connection error:", err)
	}

	// Databases and collections
	projectsDB := client.Database("projects_db")
	projectsCollection := projectsDB.Collection("projects")

	tasksDB := client.Database("tasks_db")
	usersDB := client.Database("users_db")

	// Initialize services and handlers
	projectService := &services.ProjectService{
		ProjectsCollection: projectsDB.Collection("project"),
		TasksCollection:    tasksDB.Collection("tasks"),
		UsersCollection:    usersDB.Collection("users"),
	}
	// Kreiranje jedinstvenog indeksa
	if err := createProjectNameIndex(projectsCollection); err != nil {
		log.Fatal(err)
	}
	projectHandler := handlers.NewProjectHandler(projectService)

	// Setup router
	r := mux.NewRouter()
	r.HandleFunc("/projects/{projectId}/members", projectHandler.GetProjectMembersHandler).Methods("GET")
	r.HandleFunc("/projects/{projectId}/members/{memberId}/remove", projectHandler.RemoveMemberFromProjectHandler).Methods("DELETE")
	r.HandleFunc("/projects", projectHandler.CreateProject).Methods("POST")
	r.HandleFunc("/projects/{id}/members", projectHandler.AddMemberToProjectHandler).Methods("POST")
	//r.HandleFunc("/projects/{id}/members", projectHandler.GetProjectMembersHandler).Methods("GET")
	r.HandleFunc("/users", projectHandler.GetAllUsersHandler).Methods("GET")
	r.HandleFunc("/projects", projectHandler.ListProjectsHandler).Methods("GET")
	r.HandleFunc("/projects/{id}", projectHandler.GetProjectByIDHandler).Methods("GET")
	r.HandleFunc("/projects/{id}/tasks", projectHandler.DisplayTasksForProjectHandler).Methods("GET")
	r.HandleFunc("/projects/{username}", handlers.GetProjectsByUsername(projectService)).Methods("GET")

	// Apply CORS middleware
	corsRouter := enableCORS(r)

	// Start the server
	fmt.Println("Projects service server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", corsRouter))

}

// enableCORS allows CORS for the Angular application running on port 4200
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
