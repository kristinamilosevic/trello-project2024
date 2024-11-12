package handlers

import (
	"encoding/json"
	"fmt"
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
	// Proveri Authorization header
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	// Ukloni "Bearer " prefiks
	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	// Validiraj token
	claims, err := h.JWTService.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	fmt.Println("Token validan za korisnika:", claims.Username)

	// Ekstraktovanje parametara iz URL-a
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 {
		http.Error(w, "Invalid request parameters", http.StatusBadRequest)
		return
	}

	username := pathParts[4]
	role := pathParts[5]

	// Proveri da li korisnik u tokenu odgovara korisniku koji se briše
	if username != claims.Username {
		http.Error(w, "Cannot delete another user's account", http.StatusForbidden)
		return
	}

	// Briši nalog
	err = h.UserService.DeleteAccount(username, role)
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
