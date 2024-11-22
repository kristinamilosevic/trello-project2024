package main

import (
	"log"
	"net/http"
	"notifications-service/handlers"
	"notifications-service/repositories"
	"notifications-service/services"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	logger := log.New(os.Stdout, "notifications-service ", log.LstdFlags)

	// Inicijalizacija repozitorijuma
	repo, err := repositories.NewNotificationRepo(logger)
	if err != nil {
		logger.Fatalf("Failed to initialize repository: %v", err)
	}
	defer repo.CloseSession()

	// Kreiranje tabele ako ne postoji
	repo.CreateTable()

	// Inicijalizacija servisa
	service := services.NewNotificationService(repo)

	// Inicijalizacija handler-a
	handler := handlers.NewNotificationHandler(service)

	// Postavljanje ruter-a sa prefiksom /api/notifications
	router := mux.NewRouter()
	router.HandleFunc("/api/notifications", handler.CreateNotification).Methods("POST")
	router.HandleFunc("/api/notifications", handler.GetNotificationsByUsername).Methods("GET")
	router.HandleFunc("/api/notifications/read", handler.MarkNotificationAsRead).Methods("PUT")
	router.HandleFunc("/api/notifications/delete", handler.DeleteNotification).Methods("DELETE")
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Notifications service is running"))
	}).Methods("GET")

	// Pokretanje servera
	logger.Println("Server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
