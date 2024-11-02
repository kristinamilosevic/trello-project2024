package main

import (
	"context"
	"log"
	"net/http"
	"trello-project/microservices/projects-service/handlers"
	"trello-project/microservices/projects-service/services"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

	// Definisanje routera i ruta
	http.HandleFunc("/projects/", projectHandler.RemoveMemberFromProjectHandler)

	// Pokretanje servera
	log.Println("Server running on port 8080")
	http.ListenAndServe(":8080", nil)
}
