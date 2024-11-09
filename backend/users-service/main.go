package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"trello-project/microservices/users-service/handlers"
	"trello-project/microservices/users-service/services"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// Učitavanje .env fajla
	/*err := godotenv.Load("jwt.env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	log.Println("Uspešno učitane varijable iz .env fajla")*/

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
	userService := services.NewUserService(userCollection, projectCollection, taskCollection)
	userHandler := handlers.UserHandler{UserService: userService}

	http.HandleFunc("/register", userHandler.Register)
	http.Handle("/api/auth/delete-account/", enableCORS(http.HandlerFunc(userHandler.DeleteAccountHandler)))

	srv := &http.Server{
		Addr:         ":8080",
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
