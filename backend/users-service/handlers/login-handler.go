package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
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

func (h *LoginHandler) CheckUsername(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		http.Error(w, "Username not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("true"))
}

func (h *LoginHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Proveri da li postoji korisnik sa zadatim username-om
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": req.Username}).Decode(&user)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Proveri da li email odgovara korisniku
	if user.Email != req.Email {
		http.Error(w, "Email does not match", http.StatusBadRequest)
		return
	}

	// Generiši novu lozinku i pošalji na email
	newPassword := utils.GenerateRandomPassword()
	_, err = h.UserService.UserCollection.UpdateOne(context.Background(), bson.M{"username": req.Username}, bson.M{"$set": bson.M{"password": newPassword}})
	if err != nil {
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	subject := "Your new password"
	body := fmt.Sprintf("Your new password is: %s", newPassword)
	utils.SendEmail(req.Email, subject, body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Password reset successfully"))
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
