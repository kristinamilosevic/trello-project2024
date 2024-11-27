package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"

	"go.mongodb.org/mongo-driver/bson"
)

type UserHandler struct {
	UserService *services.UserService
	JWTService  *services.JWTService
	BlackList   map[string]bool
}

// Register šalje email sa verifikacionim linkom, bez čuvanja korisnika u bazi
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	if err := h.UserService.ValidatePassword(user.Password); err != nil {
		log.Println("Password validation failed:", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password is too common. Please choose a stronger one.",
		})

		return
	}

	// Proveri da li korisničko ime već postoji
	var existingUser models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&existingUser)
	if err == nil {
		// Ako korisnik sa datim korisničkim imenom postoji, vraćamo grešku
		http.Error(w, "Username already exists", http.StatusConflict)
		return
	}

	// Nastavi sa registracijom korisnika
	err = h.UserService.RegisterUser(user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful. Check your email for confirmation link."})
}

// ConfirmEmail kreira korisnika u bazi i redirektuje na login stranicu
func (h *UserHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Token string `json:"token"`
	}

	log.Println("Primljen zahtev za potvrdu emaila")

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		log.Println("Greška pri dekodiranju JSON-a:", err)
		return
	}

	// Verifikacija tokena
	email, err := h.UserService.JWTService.VerifyEmailVerificationToken(requestData.Token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		log.Println("Nevažeći ili istekao token za email:", email, "greška:", err)
		return
	}
	log.Println("Token verifikovan za email:", email)

	// Proveri da li postoji keširan token
	tokenData, ok := h.UserService.TokenCache[email]
	if !ok {
		http.Error(w, "Token expired or not found", http.StatusUnauthorized)
		log.Println("Token nije pronađen u kešu za:", email)
		return
	}

	// Parsiranje podataka iz keša
	dataParts := strings.Split(tokenData, "|")
	if len(dataParts) < 6 {
		http.Error(w, "Invalid token data", http.StatusBadRequest)
		log.Println("Nevažeći podaci u tokenu za:", email)
		return
	}
	name := dataParts[1]
	lastName := dataParts[2]
	username := dataParts[3]
	password := dataParts[4]
	role := dataParts[5]

	// Kreiraj korisnika sa dobijenim podacima i postavi IsActive na true
	user := models.User{
		Email:    email,
		Name:     name,
		LastName: lastName,
		Username: username,
		Password: password,
		Role:     role,
		IsActive: true,
	}

	// Čuvanje korisnika u bazi
	err = h.UserService.CreateUser(user)
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		log.Println("Greška pri čuvanju korisnika u bazi:", err)
		return
	}
	log.Println("Korisnik uspešno sačuvan:", user.Email)

	// Brisanje tokena iz keša nakon uspešne verifikacije
	delete(h.UserService.TokenCache, email)

	// Redirektovanje korisnika na login stranicu
	w.Header().Set("Location", "https://localhost:4200/login")
	w.WriteHeader(http.StatusFound)
}

func (h *UserHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		log.Println("Greška pri dekodiranju zahteva:", err)
		return
	}

	log.Println("Primljen zahtev za verifikaciju korisnika:", requestData.Username, "sa kodom:", requestData.Code)

	// Provera da li postoji korisnik sa datim username-om
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": requestData.Username}).Decode(&user)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		log.Println("Korisnik nije pronađen sa username-om:", requestData.Username)
		return
	}

	log.Println("Pronađen korisnik:", user)

	// Provera da li se kod poklapa i da li je istekao
	if user.VerificationCode != requestData.Code || time.Now().After(user.VerificationExpiry) {
		log.Println("Nevažeći verifikacioni kod:", user.VerificationCode, "primljeni kod:", requestData.Code)
		// Brišemo korisnika jer kod nije validan ili je istekao
		_, delErr := h.UserService.UserCollection.DeleteOne(context.Background(), bson.M{"username": requestData.Username})
		if delErr != nil {
			log.Println("Greška prilikom brisanja korisnika:", delErr)
		}
		http.Error(w, "Invalid or expired code", http.StatusUnauthorized)
		return
	}

	// Postavljanje `IsActive` na `true` i brisanje verifikacionih podataka
	user.IsActive = true
	user.VerificationCode = ""
	user.VerificationExpiry = time.Time{} // Reset vremena isteka

	_, err = h.UserService.UserCollection.UpdateOne(context.Background(), bson.M{"username": requestData.Username}, bson.M{"$set": bson.M{
		"isActive":           true,
		"verificationCode":   "",
		"verificationExpiry": time.Time{},
	}})
	if err != nil {
		http.Error(w, "Failed to activate user", http.StatusInternalServerError)
		log.Println("Greška prilikom ažuriranja korisnika u bazi:", err)
		return
	}

	log.Println("Korisnik uspešno aktiviran:", user.Username)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User verified and saved successfully."))
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

	// Validacija tokena i izvlačenje podataka (username, role)
	claims, err := h.JWTService.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	username := claims.Username
	fmt.Printf("Request to delete account for user: %s\n", username)

	// Provera i brisanje naloga
	err = h.UserService.DeleteAccount(username)
	if err != nil {
		if strings.Contains(err.Error(), "unfinished tasks") {
			http.Error(w, err.Error(), http.StatusConflict) // HTTP 409: Conflict
		} else {
			http.Error(w, "Failed to delete account: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Uspešno brisanje
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
}

func (h *UserHandler) GetUserForCurrentSession(w http.ResponseWriter, r *http.Request) {
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

	var user models.User
	err = h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": claims.Username}).Decode(&user)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	user.Password = ""
	user.VerificationCode = ""

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		http.Error(w, "Failed to encode user data", http.StatusInternalServerError)
		return
	}
}

// ChangePassword menja lozinku korisniku
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}

	// Parsiranje podataka iz zahteva
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	// Dohvati token iz Authorization header-a
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	// Ako token počinje sa "Bearer ", ukloni ga
	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	// Validiraj token
	claims, err := h.JWTService.ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	// Provera whitelist i blacklist pravila za novu lozinku
	if err := h.UserService.ValidatePassword(requestData.NewPassword); err != nil {
		log.Println("Password validation failed:", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password is too common. Please choose a stronger one.",
		})

		return
	}

	// Pozovi servisnu metodu za promenu lozinke
	err = h.UserService.ChangePassword(claims.Username, requestData.OldPassword, requestData.NewPassword, requestData.ConfirmPassword)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Ako je uspešno, pošaljemo JSON odgovor
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{"message": "Password updated successfully"}
	json.NewEncoder(w).Encode(response)

}
