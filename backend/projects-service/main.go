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

	// Kreiranje servisa i handler-a
	projectService := service.NewProjectService(projectCollection)
	projectHandler := handler.NewProjectHandler(projectService)

	// Postavljanje ruta
	r := mux.NewRouter()
	r.HandleFunc("/projects/{id}/members", projectHandler.AddMemberToProjectHandler).Methods("POST")

	// Pokretanje servera
	fmt.Println("Server pokrenut na http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
