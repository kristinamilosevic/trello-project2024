package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	JWTService  *services.JWTService
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

//magic

type MagicLinkRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

// Funkcija za slanje magic link-a
func (h *LoginHandler) MagicLink(w http.ResponseWriter, r *http.Request) {
	// Dekodiraj telo zahteva u MagicLinkRequest strukturu
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("Error decoding request body: %v", err) // Logovanje greške pri dekodiranju
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	log.Printf("Received magic link request: %+v", req) // Logovanje podataka zahteva

	// Pronađi korisnika u bazi podataka
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": req.Username}).Decode(&user)
	if err != nil {
		log.Printf("User not found for username: %s, error: %v", req.Username, err) // Logovanje greške pri traženju korisnika
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	log.Printf("User found: %+v", user) // Logovanje podataka o korisniku

	// Proveri da li email odgovara korisniku
	if user.Email != req.Email {
		log.Printf("Email mismatch: expected %s, got %s", user.Email, req.Email) // Logovanje greške ako email ne odgovara
		http.Error(w, "Email does not match", http.StatusBadRequest)
		return
	}

	// Generiši JWT token sa username i role
	token, err := h.JWTService.GenerateMagicLinkToken(req.Username, user.Role)
	if err != nil {
		log.Printf("Error generating token: %v", err) // Logovanje greške pri generisanju tokena
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	log.Printf("Generated token: %s", token) // Logovanje generisanog tokena

	// Kreiraj magic link koji se šalje korisniku
	magicLink := fmt.Sprintf("https://localhost:4200/magic-login?token=%s", token)
	log.Printf("Generated magic link: %s", magicLink) // Logovanje magic linka

	// Slanje email-a sa magic link-om
	subject := "Your Magic Login Link"
	body := fmt.Sprintf("Click here to log in: %s", magicLink)
	if err := utils.SendEmail(req.Email, subject, body); err != nil {
		log.Printf("Failed to send email to %s: %v", req.Email, err) // Logovanje greške pri slanju emaila
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}
	log.Printf("Magic link successfully sent to: %s", req.Email) // Logovanje uspešnog slanja emaila

	// Odgovori korisniku sa uspehom
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Magic link sent successfully"))
	log.Println("Magic link request processed successfully") // Logovanje uspeha obrade zahteva
}

// Funkcija za prijavu putem magic link-a
func (h *LoginHandler) MagicLogin(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is missing", http.StatusBadRequest)
		return
	}

	// Validacija tokena
	claims, err := h.JWTService.ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Generiši novi token za sesiju
	authToken, err := h.JWTService.GenerateAuthToken(claims.Username, claims.Role)
	if err != nil {
		http.Error(w, "Failed to generate auth token", http.StatusInternalServerError)
		return
	}

	// Vraćamo odgovor sa username i role
	response := map[string]string{
		"token":    authToken,
		"username": claims.Username,
		"role":     claims.Role,
	}

	// Dodaj log ovde da proveriš šta backend šalje
	fmt.Println("Backend response:", response)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Funkcija za verifikaciju magic linka i automatsku prijavu
func (h *LoginHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Validiraj token
	claims, err := h.JWTService.ValidateToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Generiši novi token za sesiju
	authToken, err := h.JWTService.GenerateAuthToken(claims.Username, claims.Role)
	if err != nil {
		http.Error(w, "Failed to generate auth token", http.StatusInternalServerError)
		return
	}

	// Priprema odgovora
	response := map[string]string{
		"token":    authToken,
		"username": claims.Username,
		"role":     claims.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
