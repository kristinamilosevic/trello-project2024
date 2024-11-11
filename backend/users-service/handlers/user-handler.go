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
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 || pathParts[4] == "" || pathParts[5] == "" {
		http.Error(w, "Invalid request parameters", http.StatusBadRequest)
		return
	}

	userID := pathParts[4]
	role := pathParts[5]

	objectID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		http.Error(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	err = h.UserService.DeleteAccount(objectID, role)
	if err != nil {
		if err.Error() == "cannot delete manager account with active projects" ||
			err.Error() == "cannot delete member account assigned to active projects" {
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
}
