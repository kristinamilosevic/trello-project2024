package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"trello-project/microservices/workflow-service/handlers"
	"trello-project/microservices/workflow-service/logging"
	"trello-project/microservices/workflow-service/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func main() {
	// Inicijalizuj logger
	logging.InitLogger()

	err := godotenv.Load(".env")
	if err != nil {
		logging.Logger.Fatalf("Failed to load .env file: %v", err)
	}

	neo4jUri := os.Getenv("NEO4J_URI")
	neo4jUser := os.Getenv("NEO4J_USERNAME")
	neo4jPassword := os.Getenv("NEO4J_PASSWORD")

	if neo4jUri == "" || neo4jUser == "" || neo4jPassword == "" {
		logging.Logger.Fatal("Neo4j connection details are missing in .env")
	}

	driver, err := neo4j.NewDriverWithContext(neo4jUri, neo4j.BasicAuth(neo4jUser, neo4jPassword, ""))
	if err != nil {
		logging.Logger.Fatalf("Failed to create Neo4j driver: %v", err)
	}
	defer driver.Close(context.Background())

	workflowService := services.NewWorkflowService(driver)
	workflowHandler := handlers.NewWorkflowHandler(workflowService)

	router := mux.NewRouter()

	router.HandleFunc("/api/workflow/dependency", workflowHandler.AddDependency).Methods("POST")
	router.HandleFunc("/api/workflow/task-node", workflowHandler.EnsureTaskNode).Methods("POST")
	router.HandleFunc("/api/workflow/dependencies/{taskId}", workflowHandler.GetDependencies).Methods("GET")
	router.HandleFunc("/api/workflow/project/{projectId}/dependencies", workflowHandler.GetProjectDependencies).Methods("GET")
	router.HandleFunc("/api/workflow/graph/{projectId}", workflowHandler.GetWorkflowGraph).Methods("GET")

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = ":8005"
	}

	srv := &http.Server{
		Handler:      router,
		Addr:         port,
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
	}

	logging.Logger.Infof("Workflow service running on port %s", port)

	err = srv.ListenAndServe()
	if err != nil {
		logging.Logger.Fatalf("Server failed to start: %v", err)
	}
}
