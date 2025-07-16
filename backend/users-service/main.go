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

	http_client "trello-project/backend/utils"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	fmt.Println("EMAIL_PASSWORD:", os.Getenv("EMAIL_PASSWORD"))

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		log.Fatal("JWT_SECRET is not set in the environment variables")
	}

	recaptchaSecret := os.Getenv("SECRET_KEY")
	if recaptchaSecret == "" {
		log.Fatal("SECRET_KEY is not set in the environment variables")
	}

	fmt.Println("Successfully loaded variables from .env file")

	blacklistFilePath := os.Getenv("BLACKLIST_FILE_PATH")
	if blacklistFilePath == "" {
		log.Fatal("BLACKLIST_FILE_PATH is not set in the environment variables")
	}

	blackList, err := services.LoadBlackList(blacklistFilePath)
	if err != nil {
		log.Fatalf("Failed to load black list: %v", err)
	}

	mongoUsersURI := os.Getenv("MONGO_USERS_URI")
	if mongoUsersURI == "" {
		log.Fatal("MONGO_USERS_URI is not set in the environment variables")
	}

	clientOptionsUsers := options.Client().ApplyURI(mongoUsersURI)
	clientUsers, err := mongo.Connect(context.TODO(), clientOptionsUsers)
	if err != nil {
		log.Fatal(err)
	}

	err = clientUsers.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to MongoDB Users database!")

	clientOptionsProjects := options.Client().ApplyURI("mongodb://mongo-projects:27017")
	clientProjects, err := mongo.Connect(context.TODO(), clientOptionsProjects)
	if err != nil {
		log.Fatal(err)
	}
	err = clientProjects.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB Projects database!")

	clientOptionsTasks := options.Client().ApplyURI("mongodb://mongo-tasks:27017")
	clientTasks, err := mongo.Connect(context.TODO(), clientOptionsTasks)
	if err != nil {
		log.Fatal(err)
	}
	err = clientTasks.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB Tasks database!")

	mongoUsersDB := os.Getenv("MONGO_USERS_DB")
	if mongoUsersDB == "" {
		log.Fatal("MONGO_USERS_DB is not set in the environment variables")
	}

	mongoUsersCollection := os.Getenv("MONGO_USERS_COLLECTION")
	if mongoUsersCollection == "" {
		log.Fatal("MONGO_USERS_COLLECTION is not set in the environment variables")
	}

	userCollection := clientUsers.Database(mongoUsersDB).Collection(mongoUsersCollection)

	jwtService := services.NewJWTService(secretKey)
	httpClient := http_client.NewHTTPClient()

	projectsBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ProjectsServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("Circuit Breaker '%s' changed from '%s' to '%s'\n", name, from.String(), to.String())
		},
	})

	tasksBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "TasksServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("Circuit Breaker '%s' changed from '%s' to '%s'\n", name, from.String(), to.String())
		},
	})

	userService := services.NewUserService(userCollection, jwtService, blackList, httpClient, projectsBreaker, tasksBreaker)

	userHandler := handlers.UserHandler{UserService: userService, JWTService: jwtService, BlackList: blackList}
	loginHandler := handlers.LoginHandler{UserService: userService}

	mux := mux.NewRouter()
	mux.HandleFunc("/api/users/register", userHandler.Register).Methods("POST")
	mux.HandleFunc("/api/users/confirm", userHandler.ConfirmEmail).Methods("POST")
	mux.HandleFunc("/api/users/verify-code", userHandler.VerifyCode).Methods("POST")
	mux.HandleFunc("/api/users/login", loginHandler.Login).Methods("POST")
	mux.HandleFunc("/api/users/check-username", loginHandler.CheckUsername).Methods("POST")
	mux.HandleFunc("/api/users/forgot-password", loginHandler.ForgotPassword).Methods("POST")
	mux.HandleFunc("/api/users/reset-password", loginHandler.ResetPassword).Methods("POST")
	mux.HandleFunc("/api/users/auth/delete-account/{username}", userHandler.DeleteAccountHandler).Methods("DELETE")
	mux.HandleFunc("/api/users/magic-link", loginHandler.MagicLink).Methods("POST")
	mux.HandleFunc("/api/users/magic-login", loginHandler.MagicLogin).Methods("POST")
	mux.HandleFunc("/api/users/verify-magic-link", loginHandler.VerifyMagicLink).Methods("POST")
	mux.HandleFunc("/api/users/users-profile", userHandler.GetUserForCurrentSession).Methods("GET")
	mux.HandleFunc("/api/users/change-password", userHandler.ChangePassword).Methods("POST")
	//mux.HandleFunc("/api/users/members", userHandler.GetAllMembers)
	mux.HandleFunc("/api/users/member/{username}", userHandler.GetMemberByUsernameHandler).Methods("GET")
	mux.HandleFunc("/api/users/projects/{projectId}/members", userHandler.GetMembersByProjectIDHandler).Methods("GET")
	mux.HandleFunc("/api/users/members", userHandler.GetAllMembers).Methods("GET")
	mux.HandleFunc("/api/users/role/{username}", userHandler.GetRoleByUsernameHandler).Methods("GET")
	mux.HandleFunc("/api/users/id/{username}", userHandler.GetIDByUsernameHandler).Methods("GET")
	mux.HandleFunc("/api/users/member/id/{id}", userHandler.GetMemberByIDHandler).Methods("GET")

	finalHandler := enableCORS(mux)

	startUserCleanupJob(userService)

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		log.Fatal("SERVER_PORT is not set in the environment variables")
	}

	srv := &http.Server{
		Addr:         serverPort,
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	fmt.Println("Server is running on port", serverPort)
	log.Fatal(srv.ListenAndServe())

}

func startUserCleanupJob(userService *services.UserService) {
	go func() {
		for {
			log.Println("Pokrećem proveru za brisanje neaktivnih korisnika sa isteklim rokom za verifikaciju...")
			userService.DeleteExpiredUnverifiedUsers()
			log.Println("Završena provera za brisanje neaktivnih korisnika.")
			time.Sleep(5 * time.Minute)
		}
	}()
}
