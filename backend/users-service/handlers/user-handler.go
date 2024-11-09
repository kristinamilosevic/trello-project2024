package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UserHandler struct {
	UserService *services.UserService
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	err := h.UserService.RegisterUser(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Registration successful. Check your email for confirmation link."))
}

func (h *UserHandler) DeleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	// Ekstraktovanje ID-a iz URL-a
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 || pathParts[4] == "" {
		http.Error(w, "Invalid manager ID", http.StatusBadRequest)
		return
	}

	managerID := pathParts[4]
	objectID, err := primitive.ObjectIDFromHex(managerID)
	if err != nil {
		http.Error(w, "Invalid manager ID format", http.StatusBadRequest)
		return
	}

	err = h.UserService.DeleteManagerAccount(objectID)
	if err != nil {
		if err.Error() == "user does not exist in the database" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if err.Error() == "cannot delete account with active projects" {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
}
