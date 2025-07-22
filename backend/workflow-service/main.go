package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"trello-project/microservices/workflow-service/handlers"
	"trello-project/microservices/workflow-service/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", os.Getenv("CORS_ORIGIN"))
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
		log.Fatal("Error loading .env file")
	}

	neo4jUri := os.Getenv("NEO4J_URI")
	neo4jUser := os.Getenv("NEO4J_USERNAME")
	neo4jPassword := os.Getenv("NEO4J_PASSWORD")

	if neo4jUri == "" || neo4jUser == "" || neo4jPassword == "" {
		log.Fatal("Neo4j connection details are missing in .env")
	}

	driver, err := neo4j.NewDriverWithContext(neo4jUri, neo4j.BasicAuth(neo4jUser, neo4jPassword, ""))
	if err != nil {
		log.Fatalf("Failed to create Neo4j driver: %v", err)
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

	fmt.Println("Workflow service running on port", port)
	log.Fatal(srv.ListenAndServe())
}
