package handlers

import (
	"encoding/json"
	"net/http"

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
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Verifikacija tokena i preuzimanje emaila
	email, err := h.UserService.JWTService.VerifyEmailVerificationToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Kreiranje korisnika sa preuzetim email-om
	user := models.User{
		Email: email,
	}

	// Čuvanje korisnika u bazi
	err = h.UserService.CreateUser(user)
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	// Redirektuje korisnika na frontend bez prikazivanja tokena
	http.Redirect(w, r, "http://localhost:4200/projects-list", http.StatusFound)
}
