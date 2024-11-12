package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"
)

type UserHandler struct {
	UserService *services.UserService
	JWTService  *services.JWTService
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
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	claims, err := h.JWTService.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	username := claims.Username
	role := claims.Role

	if role == "manager" {
		canDelete, err := h.UserService.CanDeleteManagerAccountByUsername(username)
		if err != nil || !canDelete {
			http.Error(w, "Cannot delete manager account with active tasks", http.StatusConflict)
			return
		}
	} else if role == "member" {
		canDelete, err := h.UserService.CanDeleteMemberAccountByUsername(username)
		if err != nil || !canDelete {
			http.Error(w, "Cannot delete member account with active tasks", http.StatusConflict)
			return
		}
	}

	err = h.UserService.DeleteAccount(username)
	if err != nil {
		http.Error(w, "Failed to delete account", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
}
