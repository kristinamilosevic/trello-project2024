package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"trello-project/microservices/users-service/handlers"
	"trello-project/microservices/users-service/services"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Učitavanje .env fajla
	err := godotenv.Load("jwt.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	log.Println("Successfully loaded variables from .env file")

	// Konektovanje na MongoDB
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB!")

	// Kolekcija korisnika
	userCollection := client.Database("users").Collection("users")
	userService := services.NewUserService(userCollection)

	// Inicijalizacija handlera
	userHandler := handlers.UserHandler{UserService: userService}
	loginHandler := handlers.LoginHandler{UserService: userService}

	// Postavi rute
	http.HandleFunc("/register", userHandler.Register)
	http.HandleFunc("/login", loginHandler.Login)

	// Pokreni server
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	fmt.Println("Server is running on port 8080")
	log.Fatal(srv.ListenAndServe())
}
