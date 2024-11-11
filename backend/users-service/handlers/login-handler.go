package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"trello-project/microservices/users-service/services"
	"trello-project/microservices/users-service/utils"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type LoginHandler struct {
	UserService *services.UserService
}

type ForgotPasswordRequest struct {
	Username string `json:"username"`
}

// Funkcija za reset lozinke
func (h *LoginHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Generišemo token za reset lozinke
	token, err := h.UserService.JWTService.GenerateEmailVerificationToken(req.Username)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	resetLink := fmt.Sprintf("http://localhost:4200/reset-password?token=%s", token)
	subject := "Reset your password"
	body := fmt.Sprintf("Click the link to reset your password: %s", resetLink)
	utils.SendEmail(req.Username, subject, body)

	w.WriteHeader(http.StatusOK)
}

// Validacija unosa korisničkih podataka
func validateCredentials(username, password string) bool {
	if len(username) < 3 || len(username) > 20 {
		return false
	}
	if len(password) < 6 || len(password) > 20 {
		return false
	}
	return true
}

// Funkcija za prijavu korisnika
func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validacija unosa
	if !validateCredentials(req.Username, req.Password) {
		http.Error(w, "Invalid credentials format", http.StatusBadRequest)
		return
	}

	// Pozivamo funkciju LoginUser iz servisa koja vraća korisnika i token
	user, token, err := h.UserService.LoginUser(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Priprema odgovora
	response := LoginResponse{
		Token:    token,
		Username: user.Username,
		Role:     user.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
