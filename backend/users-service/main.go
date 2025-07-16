package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"trello-project/microservices/users-service/handlers"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
	"trello-project/microservices/users-service/services"

	http_client "trello-project/backend/utils" // Pretpostavljam da je ovo putanja do http klijenta

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
			logging.Logger.Debug("Event ID: CORS_PREFLIGHT_OK, Description: Handled CORS preflight request.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	logging.InitLogger()
	logging.Logger.Info("Event ID: SERVICE_STARTUP, Description: Starting Users Service application.")

	err := godotenv.Load(".env")
	if err != nil {
		logging.Logger.Fatalf("Event ID: ENV_LOAD_FAILED, Description: Error loading .env file: %v", err)
	}
	logging.Logger.Debugf("Event ID: ENV_LOADED, Description: EMAIL_PASSWORD: %s", os.Getenv("EMAIL_PASSWORD")) // Be cautious with logging sensitive info at higher levels

	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		logging.Logger.Fatalf("Event ID: JWT_SECRET_MISSING, Description: JWT_SECRET is not set in the environment variables.")
	}
	logging.Logger.Debug("Event ID: JWT_SECRET_CHECKED, Description: JWT_SECRET environment variable is set.")

	recaptchaSecret := os.Getenv("SECRET_KEY")
	if recaptchaSecret == "" {
		logging.Logger.Fatalf("Event ID: RECAPTCHA_SECRET_MISSING, Description: SECRET_KEY (reCAPTCHA) is not set in the environment variables.")
	}
	logging.Logger.Debug("Event ID: RECAPTCHA_SECRET_CHECKED, Description: SECRET_KEY environment variable is set.")

	logging.Logger.Info("Event ID: ENV_VARS_LOADED_SUCCESS, Description: Successfully loaded variables from .env file.")

	blacklistFilePath := os.Getenv("BLACKLIST_FILE_PATH")
	if blacklistFilePath == "" {
		logging.Logger.Fatalf("Event ID: BLACKLIST_PATH_MISSING, Description: BLACKLIST_FILE_PATH is not set in the environment variables.")
	}
	logging.Logger.Debugf("Event ID: BLACKLIST_PATH_CHECKED, Description: BLACKLIST_FILE_PATH: %s", blacklistFilePath)

	blackList, err := services.LoadBlackList(blacklistFilePath)
	if err != nil {
		logging.Logger.Fatalf("Event ID: BLACKLIST_LOAD_FAILED, Description: Failed to load black list from '%s': %v", blacklistFilePath, err)
	}
	logging.Logger.Infof("Event ID: BLACKLIST_LOADED, Description: Successfully loaded password blacklist with %d entries.", len(blackList))

	mongoUsersURI := os.Getenv("MONGO_USERS_URI")
	if mongoUsersURI == "" {
		logging.Logger.Fatalf("Event ID: MONGO_USERS_URI_MISSING, Description: MONGO_USERS_URI is not set in the environment variables.")
	}
	logging.Logger.Debug("Event ID: MONGO_USERS_URI_CHECKED, Description: MONGO_USERS_URI environment variable is set.")

	// Connect to MongoDB Users database
	clientOptionsUsers := options.Client().ApplyURI(mongoUsersURI)
	clientUsers, err := mongo.Connect(context.TODO(), clientOptionsUsers)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_USERS_CONNECT_FAILED, Description: Failed to connect to MongoDB Users database: %v", err)
	}

	err = clientUsers.Ping(context.TODO(), nil)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_USERS_PING_FAILED, Description: Failed to ping MongoDB Users database: %v", err)
	}
	logging.Logger.Info("Event ID: MONGO_USERS_CONNECTED, Description: Connected to MongoDB Users database!")

	// Connect to MongoDB Projects database (assuming this is for other services, if it's not directly used here, it might be redundant)
	clientOptionsProjects := options.Client().ApplyURI("mongodb://mongo-projects:27017")
	clientProjects, err := mongo.Connect(context.TODO(), clientOptionsProjects)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_PROJECTS_CONNECT_FAILED, Description: Failed to connect to MongoDB Projects database: %v", err)
	}
	err = clientProjects.Ping(context.TODO(), nil)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_PROJECTS_PING_FAILED, Description: Failed to ping MongoDB Projects database: %v", err)
	}
	logging.Logger.Info("Event ID: MONGO_PROJECTS_CONNECTED, Description: Connected to MongoDB Projects database!")

	// Connect to MongoDB Tasks database (assuming this is for other services)
	clientOptionsTasks := options.Client().ApplyURI("mongodb://mongo-tasks:27017")
	clientTasks, err := mongo.Connect(context.TODO(), clientOptionsTasks)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_TASKS_CONNECT_FAILED, Description: Failed to connect to MongoDB Tasks database: %v", err)
	}
	err = clientTasks.Ping(context.TODO(), nil)
	if err != nil {
		logging.Logger.Fatalf("Event ID: MONGO_TASKS_PING_FAILED, Description: Failed to ping MongoDB Tasks database: %v", err)
	}
	logging.Logger.Info("Event ID: MONGO_TASKS_CONNECTED, Description: Connected to MongoDB Tasks database!")

	mongoUsersDB := os.Getenv("MONGO_USERS_DB")
	if mongoUsersDB == "" {
		logging.Logger.Fatalf("Event ID: MONGO_USERS_DB_MISSING, Description: MONGO_USERS_DB is not set in the environment variables.")
	}
	logging.Logger.Debugf("Event ID: MONGO_USERS_DB_CHECKED, Description: MONGO_USERS_DB: %s", mongoUsersDB)

	mongoUsersCollection := os.Getenv("MONGO_USERS_COLLECTION")
	if mongoUsersCollection == "" {
		logging.Logger.Fatalf("Event ID: MONGO_USERS_COLLECTION_MISSING, Description: MONGO_USERS_COLLECTION is not set in the environment variables.")
	}
	logging.Logger.Debugf("Event ID: MONGO_USERS_COLLECTION_CHECKED, Description: MONGO_USERS_COLLECTION: %s", mongoUsersCollection)

	userCollection := clientUsers.Database(mongoUsersDB).Collection(mongoUsersCollection)
	logging.Logger.Info("Event ID: USER_COLLECTION_INIT, Description: Initialized MongoDB user collection.")

	jwtService := services.NewJWTService(secretKey)
	httpClient := http_client.NewHTTPClient()
	logging.Logger.Info("Event ID: SERVICES_INIT, Description: Initialized JWTService and HTTPClient.")

	// Circuit breaker for Projects Service
	projectsBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "ProjectsServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Warnf("Event ID: CB_STATE_CHANGE, Description: Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})
	logging.Logger.Info("Event ID: PROJECTS_CB_INIT, Description: Initialized Projects Service Circuit Breaker.")

	// Circuit breaker for Tasks Service
	tasksBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "TasksServiceCB",
		MaxRequests: 1,
		Timeout:     2 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logging.Logger.Warnf("Event ID: CB_STATE_CHANGE, Description: Circuit Breaker '%s' changed from '%s' to '%s'", name, from.String(), to.String())
		},
	})
	logging.Logger.Info("Event ID: TASKS_CB_INIT, Description: Initialized Tasks Service Circuit Breaker.")

	userService := services.NewUserService(userCollection, jwtService, blackList, httpClient, projectsBreaker, tasksBreaker)
	logging.Logger.Info("Event ID: USER_SERVICE_INIT, Description: Initialized UserService.")

	userHandler := handlers.UserHandler{UserService: userService, JWTService: jwtService, BlackList: blackList}
	loginHandler := handlers.LoginHandler{UserService: userService}
	logging.Logger.Info("Event ID: HANDLERS_INIT, Description: Initialized UserHandler and LoginHandler.")

	router := mux.NewRouter()
	router.HandleFunc("/api/users/register", userHandler.Register).Methods("POST")
	router.HandleFunc("/api/users/confirm", userHandler.ConfirmEmail).Methods("POST")
	router.HandleFunc("/api/users/verify-code", userHandler.VerifyCode).Methods("POST")
	router.HandleFunc("/api/users/login", loginHandler.Login).Methods("POST")
	router.HandleFunc("/api/users/check-username", loginHandler.CheckUsername).Methods("POST")
	router.HandleFunc("/api/users/forgot-password", loginHandler.ForgotPassword).Methods("POST")
	router.HandleFunc("/api/users/reset-password", loginHandler.ResetPassword).Methods("POST")
	router.HandleFunc("/api/users/auth/delete-account/{username}", userHandler.DeleteAccountHandler).Methods("DELETE")
	router.HandleFunc("/api/users/magic-link", loginHandler.MagicLink).Methods("POST")
	router.HandleFunc("/api/users/magic-login", loginHandler.MagicLogin).Methods("POST")
	router.HandleFunc("/api/users/verify-magic-link", loginHandler.VerifyMagicLink).Methods("POST")
	router.HandleFunc("/api/users/users-profile", userHandler.GetUserForCurrentSession).Methods("GET")
	router.HandleFunc("/api/users/change-password", userHandler.ChangePassword).Methods("POST")
	router.HandleFunc("/api/users/member/{username}", userHandler.GetMemberByUsernameHandler).Methods("GET")
	router.HandleFunc("/api/users/projects/{projectId}/members", userHandler.GetMembersByProjectIDHandler).Methods("GET")
	router.HandleFunc("/api/users/members", userHandler.GetAllMembers).Methods("GET")
	router.HandleFunc("/api/users/role/{username}", userHandler.GetRoleByUsernameHandler).Methods("GET")
	router.HandleFunc("/api/users/id/{username}", userHandler.GetIDByUsernameHandler).Methods("GET")
	router.HandleFunc("/api/users/member/id/{id}", userHandler.GetMemberByIDHandler).Methods("GET")
	logging.Logger.Info("Event ID: ROUTES_REGISTERED, Description: Registered all API routes.")

	finalHandler := enableCORS(router)
	logging.Logger.Info("Event ID: CORS_ENABLED, Description: CORS middleware enabled.")

	startUserCleanupJob(userService)
	logging.Logger.Info("Event ID: CLEANUP_JOB_STARTED, Description: Started background job for deleting expired unverified users.")

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		logging.Logger.Fatalf("Event ID: SERVER_PORT_MISSING, Description: SERVER_PORT is not set in the environment variables.")
	}
	logging.Logger.Debugf("Event ID: SERVER_PORT_CHECKED, Description: Server port set to: %s", serverPort)

	srv := &http.Server{
		Addr:         serverPort,
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logging.Logger.Infof("Event ID: SERVER_LISTENING, Description: Server is running on port %s", serverPort)
	logging.Logger.Fatal("Event ID: SERVER_SHUTDOWN, Description: HTTP server stopped: ", srv.ListenAndServe())
}

func startUserCleanupJob(userService *services.UserService) {
	go func() {
		for {
			logging.Logger.Info("Event ID: USER_CLEANUP_JOB_RUNNING, Description: Initiating cleanup check for expired unverified users...")
			userService.DeleteExpiredUnverifiedUsers()
			logging.Logger.Info("Event ID: USER_CLEANUP_JOB_COMPLETED, Description: Completed cleanup check for inactive users. Sleeping for 5 minutes.")
			time.Sleep(5 * time.Minute)
		}
	}()
}
