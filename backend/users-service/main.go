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

// CORS Middleware funkcija
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

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

	// Kreiranje novog multiplexer-a i dodavanje ruta
	mux := http.NewServeMux()
	mux.HandleFunc("/register", userHandler.Register)
	mux.HandleFunc("/login", loginHandler.Login)
	mux.HandleFunc("/forgot-password", loginHandler.ForgotPassword)

	// Primena CORS i JWT Middleware-a
	finalHandler := enableCORS(mux)

	// Pokretanje servera
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println("Server is running on port 8080")
	log.Fatal(srv.ListenAndServe())
}
