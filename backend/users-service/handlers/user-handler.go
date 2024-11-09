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
}

// Register šalje email sa verifikacionim linkom, bez čuvanja korisnika u bazi
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

// ConfirmEmail kreira korisnika u bazi i redirektuje na frontend
func (h *UserHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Email string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Proverite da li token i podaci korisnika postoje u kešu
	tokenData, ok := h.UserService.TokenCache[requestData.Email]
	if !ok {
		http.Error(w, "Token expired or not found", http.StatusUnauthorized)
		return
	}

	// Podelite podatke iz keša
	dataParts := strings.Split(tokenData, "|")
	if len(dataParts) < 6 {
		http.Error(w, "Invalid token data", http.StatusBadRequest)
		return
	}
	token := dataParts[0]
	name := dataParts[1]
	lastName := dataParts[2]
	username := dataParts[3]
	password := dataParts[4]
	role := dataParts[5]

	// Verifikujte token
	email, err := h.UserService.JWTService.VerifyEmailVerificationToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Kreirajte korisnika sa podacima iz keša
	user := models.User{
		Email:    email,
		Name:     name,
		LastName: lastName,
		Username: username,
		Password: password,
		Role:     role,
	}

	// Sačuvajte korisnika u bazi
	err = h.UserService.CreateUser(user)
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	// Izbrišite token iz keša
	delete(h.UserService.TokenCache, requestData.Email)

	// Redirektujte korisnika
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Registration confirmed. You may now log in once the login page is ready."))
}
