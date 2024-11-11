package handlers

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/users-service/services"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type LoginHandler struct {
	UserService *services.UserService
}

func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Pozivamo funkciju LoginUser koja sada vraća korisnika i token
	user, token, err := h.UserService.LoginUser(req.Email, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Kreiramo odgovor koristeći informacije iz `user` objekta
	response := LoginResponse{
		Token: token,
		Email: user.Email,
		Role:  user.Role,
	}

	json.NewEncoder(w).Encode(response)
}
