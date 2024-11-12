package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
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

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		log.Fatal("JWT_SECRET is not set in the environment variables")
	}

	fmt.Println("Successfully loaded variables from .env file")

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

	projectCollection := client.Database("projects_db").Collection("project")
	taskCollection := client.Database("tasks_db").Collection("tasks")
	jwtService := services.NewJWTService(secretKey)
	userService := services.NewUserService(userCollection, projectCollection, taskCollection, jwtService)

	userHandler := handlers.UserHandler{UserService: userService, JWTService: jwtService}
	loginHandler := handlers.LoginHandler{UserService: userService}

	http.HandleFunc("/register", userHandler.Register)

	// Kreiranje novog multiplexer-a i dodavanje ruta
	mux := http.NewServeMux()
	mux.HandleFunc("/register", userHandler.Register)
	mux.HandleFunc("/login", loginHandler.Login)
	mux.HandleFunc("/check-username", loginHandler.CheckUsername)
	mux.HandleFunc("/forgot-password", loginHandler.ForgotPassword)
	mux.HandleFunc("/api/auth/delete-account/", userHandler.DeleteAccountHandler)

	mux.HandleFunc("/magic-link", loginHandler.MagicLink)
	mux.HandleFunc("/magic-login", loginHandler.MagicLogin)
	mux.HandleFunc("/verify-magic-link", loginHandler.VerifyMagicLink)

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

// CORS Middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
