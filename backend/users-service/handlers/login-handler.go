package handlers

import (
	"context"
	"encoding/json"
	"fmt" // Ostavljamo ga, ali nećemo ga koristiti za logovanje aplikacije
	"net/http"
	"trello-project/microservices/users-service/logging"
	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/bcrypt"
)

type LoginRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captchaToken"`
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
	logging.Logger.Debugf("Event ID: CHECK_USERNAME_START, Description: Starting CheckUsername handler.")
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Event ID: CHECK_USERNAME_AUTH_FAILED, Description: Access forbidden for CheckUsername: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		logging.Logger.Warn("Event ID: CHECK_USERNAME_BAD_REQUEST, Description: Username is required for CheckUsername.")
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	logging.Logger.Debugf("Event ID: CHECK_USERNAME_DB_QUERY, Description: Checking existence of username: %s", username)
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Infof("Event ID: CHECK_USERNAME_NOT_FOUND, Description: Username '%s' not found: %v", username, err)
		http.Error(w, "Username not found", http.StatusNotFound)
		return
	}

	logging.Logger.Infof("Event ID: CHECK_USERNAME_SUCCESS, Description: Username '%s' found successfully.", username)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("true"))
}

func (h *LoginHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: FORGOT_PASSWORD_START, Description: Starting ForgotPassword handler.")
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Event ID: FORGOT_PASSWORD_INVALID_REQUEST, Description: Invalid request for ForgotPassword: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	logging.Logger.Infof("Event ID: FORGOT_PASSWORD_EMAIL_SEND, Description: Attempting to send password reset link for username: %s, email: %s", req.Username, req.Email)
	err := h.UserService.SendPasswordResetLink(req.Username, req.Email)
	if err != nil {
		logging.Logger.Errorf("Event ID: FORGOT_PASSWORD_SEND_FAILED, Description: Failed to send reset link for %s: %v", req.Username, err)
		http.Error(w, "Failed to send reset link: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: FORGOT_PASSWORD_SUCCESS, Description: Password reset link sent successfully to %s.", req.Email)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Password reset link sent successfully"))
}

func validateCredentials(username, password string) bool {
	logging.Logger.Debugf("Event ID: VALIDATE_CREDENTIALS, Description: Validating credentials format.")
	if len(username) < 3 || len(username) > 20 {
		logging.Logger.Warnf("Event ID: VALIDATE_CREDENTIALS_USERNAME_LENGTH, Description: Invalid username length: %d", len(username))
		return false
	}
	if len(password) < 6 || len(password) > 20 {
		logging.Logger.Warnf("Event ID: VALIDATE_CREDENTIALS_PASSWORD_LENGTH, Description: Invalid password length: %d", len(password))
		return false
	}
	logging.Logger.Debug("Event ID: VALIDATE_CREDENTIALS_SUCCESS, Description: Credentials format is valid.")
	return true
}

func (h *LoginHandler) Login(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: LOGIN_HANDLER_START, Description: Starting Login handler.")
	if r.Method != http.MethodPost {
		logging.Logger.Warnf("Event ID: LOGIN_METHOD_NOT_ALLOWED, Description: Method not allowed for Login: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Event ID: LOGIN_INVALID_REQUEST_FORMAT, Description: Invalid request format for Login: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validacija CAPTCHA tokena
	if req.CaptchaToken == "" {
		logging.Logger.Warn("Event ID: LOGIN_MISSING_CAPTCHA, Description: Missing CAPTCHA token.")
		http.Error(w, "Missing CAPTCHA token", http.StatusBadRequest)
		return
	}

	isValid, err := utils.VerifyCaptcha(req.CaptchaToken)
	if err != nil || !isValid {
		logging.Logger.Warnf("Event ID: LOGIN_INVALID_CAPTCHA, Description: Invalid CAPTCHA token or error: %v", err)
		http.Error(w, "Invalid CAPTCHA token", http.StatusForbidden)
		return
	}

	if !validateCredentials(req.Username, req.Password) {
		logging.Logger.Warn("Event ID: LOGIN_INVALID_CREDENTIALS_FORMAT, Description: Invalid credentials format for Login.")
		http.Error(w, "Invalid credentials format", http.StatusBadRequest)
		return
	}

	// Pozivamo funkciju LoginUser iz servisa koja vraća korisnika i token
	user, token, err := h.UserService.LoginUser(req.Username, req.Password)
	if err != nil {
		logging.Logger.Errorf("Event ID: LOGIN_FAILED, Description: User login failed for '%s': %v", req.Username, err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	response := LoginResponse{
		Token:    token,
		Username: user.Username,
		Role:     user.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
	logging.Logger.Infof("Event ID: LOGIN_SUCCESS, Description: User '%s' logged in successfully with role '%s'.", user.Username, user.Role)
}

//magic

type MagicLinkRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (h *LoginHandler) MagicLink(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: MAGIC_LINK_HANDLER_START, Description: Starting MagicLink handler.")
	// Dekodiraj telo zahteva u MagicLinkRequest strukturu
	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Errorf("Event ID: MAGIC_LINK_DECODE_ERROR, Description: Error decoding request body for MagicLink: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	logging.Logger.Infof("Event ID: MAGIC_LINK_REQUEST_RECEIVED, Description: Received magic link request for username: %s, email: %s", req.Username, req.Email)

	// Pronađi korisnika u bazi podataka
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": req.Username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: MAGIC_LINK_USER_NOT_FOUND, Description: User not found for username '%s', error: %v", req.Username, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	logging.Logger.Debugf("Event ID: MAGIC_LINK_USER_FOUND, Description: User '%s' found in database.", user.Username)

	if user.Email != req.Email {
		logging.Logger.Warnf("Event ID: MAGIC_LINK_EMAIL_MISMATCH, Description: Email mismatch for user '%s': expected %s, got %s", req.Username, user.Email, req.Email)
		http.Error(w, "Email does not match", http.StatusBadRequest)
		return
	}

	// Generiši JWT token sa username i role
	token, err := h.JWTService.GenerateMagicLinkToken(req.Username, user.Role)
	if err != nil {
		logging.Logger.Errorf("Event ID: MAGIC_LINK_TOKEN_GEN_FAILED, Description: Error generating magic link token for '%s': %v", req.Username, err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: MAGIC_LINK_TOKEN_GENERATED, Description: Generated magic link token for '%s'.", req.Username)

	// Kreiraj magic link koji se šalje korisniku
	magicLink := fmt.Sprintf("https://localhost:4200/magic-login?token=%s", token)
	logging.Logger.Debugf("Event ID: MAGIC_LINK_GENERATED_URL, Description: Generated magic link URL for '%s'.", req.Username)

	subject := "Your Magic Login Link"
	body := fmt.Sprintf("Click here to log in: %s", magicLink)
	if err := utils.SendEmail(req.Email, subject, body); err != nil {
		logging.Logger.Errorf("Event ID: MAGIC_LINK_EMAIL_SEND_FAILED, Description: Failed to send magic link email to '%s': %v", req.Email, err)
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: MAGIC_LINK_EMAIL_SENT_SUCCESS, Description: Magic link successfully sent to: %s", req.Email)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Magic link sent successfully"))
	logging.Logger.Infof("Event ID: MAGIC_LINK_REQUEST_PROCESSED, Description: Magic link request processed successfully for '%s'.", req.Username)
}

// Funkcija za prijavu putem magic link-a
func (h *LoginHandler) MagicLogin(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: MAGIC_LOGIN_HANDLER_START, Description: Starting MagicLogin handler.")
	token := r.URL.Query().Get("token")
	if token == "" {
		logging.Logger.Warn("Event ID: MAGIC_LOGIN_MISSING_TOKEN, Description: Magic login token is missing.")
		http.Error(w, "Token is missing", http.StatusBadRequest)
		return
	}

	claims, err := h.JWTService.ValidateToken(token)
	if err != nil {
		logging.Logger.Warnf("Event ID: MAGIC_LOGIN_INVALID_TOKEN, Description: Invalid or expired magic login token: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: MAGIC_LOGIN_TOKEN_VALIDATED, Description: Magic login token validated for user: %s", claims.Username)

	//novi token za sesiju
	authToken, err := h.JWTService.GenerateAuthToken(claims.Username, claims.Role)
	if err != nil {
		logging.Logger.Errorf("Event ID: MAGIC_LOGIN_AUTH_TOKEN_GEN_FAILED, Description: Failed to generate auth token for magic login: %v", err)
		http.Error(w, "Failed to generate auth token", http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: MAGIC_LOGIN_AUTH_TOKEN_GENERATED, Description: Auth token generated for magic login user: %s", claims.Username)

	response := map[string]string{
		"token":    authToken,
		"username": claims.Username,
		"role":     claims.Role,
	}

	// Zamenjeno fmt.Println sa logging.Logger
	logging.Logger.Infof("Event ID: MAGIC_LOGIN_RESPONSE, Description: Backend response for MagicLogin: Token Length: %d, Username: %s, Role: %s", len(response["token"]), response["username"], response["role"])

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logging.Logger.Infof("Event ID: MAGIC_LOGIN_SUCCESS, Description: Magic login successful for user: %s.", claims.Username)
}

// Funkcija za verifikaciju magic linka i automatsku prijavu
func (h *LoginHandler) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: VERIFY_MAGIC_LINK_START, Description: Starting VerifyMagicLink handler.")
	token := r.URL.Query().Get("token")
	if token == "" {
		logging.Logger.Warn("Event ID: VERIFY_MAGIC_LINK_MISSING_TOKEN, Description: Missing magic link token for verification.")
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	claims, err := h.JWTService.ValidateToken(token)
	if err != nil {
		logging.Logger.Warnf("Event ID: VERIFY_MAGIC_LINK_INVALID_TOKEN, Description: Invalid or expired magic link token during verification: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: VERIFY_MAGIC_LINK_TOKEN_VALIDATED, Description: Magic link token validated for user: %s", claims.Username)

	//novi token za sesiju
	authToken, err := h.JWTService.GenerateAuthToken(claims.Username, claims.Role)
	if err != nil {
		logging.Logger.Errorf("Event ID: VERIFY_MAGIC_LINK_AUTH_TOKEN_GEN_FAILED, Description: Failed to generate auth token after magic link verification: %v", err)
		http.Error(w, "Failed to generate auth token", http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: VERIFY_MAGIC_LINK_AUTH_TOKEN_GENERATED, Description: Auth token generated after magic link verification for user: %s", claims.Username)

	response := map[string]string{
		"token":    authToken,
		"username": claims.Username,
		"role":     claims.Role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logging.Logger.Infof("Event ID: VERIFY_MAGIC_LINK_SUCCESS, Description: Magic link verified and user '%s' authenticated successfully.", claims.Username)
}

func (h *LoginHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debugf("Event ID: RESET_PASSWORD_HANDLER_START, Description: Starting ResetPassword handler.")
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}

	// Decode JSON zahteva
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Event ID: RESET_PASSWORD_INVALID_REQUEST, Description: Invalid request for ResetPassword: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Proveri validnost tokena
	claims, err := h.JWTService.ValidateToken(req.Token)
	if err != nil {
		logging.Logger.Warnf("Event ID: RESET_PASSWORD_INVALID_TOKEN, Description: Invalid or expired reset password token: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: RESET_PASSWORD_TOKEN_VALIDATED, Description: Reset password token validated for user: %s", claims.Username)

	// Validacija nove lozinke
	if err := h.UserService.ValidatePassword(req.NewPassword); err != nil {
		logging.Logger.Warnf("Event ID: RESET_PASSWORD_INVALID_PASSWORD, Description: New password validation failed for '%s': %v", claims.Username, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logging.Logger.Debugf("Event ID: RESET_PASSWORD_NEW_PASSWORD_VALIDATED, Description: New password validated for user: %s", claims.Username)

	// Hashuj novu lozinku
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		logging.Logger.Errorf("Event ID: RESET_PASSWORD_HASH_FAILED, Description: Failed to hash new password for '%s': %v", claims.Username, err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: RESET_PASSWORD_HASHED, Description: New password hashed for user: %s", claims.Username)

	// Ažuriraj lozinku korisnika
	_, err = h.UserService.UserCollection.UpdateOne(
		context.Background(),
		bson.M{"username": claims.Username},
		bson.M{"$set": bson.M{"password": string(hashedPassword)}},
	)
	if err != nil {
		logging.Logger.Errorf("Event ID: RESET_PASSWORD_UPDATE_FAILED, Description: Failed to update password for '%s': %v", claims.Username, err)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: RESET_PASSWORD_SUCCESS, Description: Password reset successfully for user: %s.", claims.Username)

	// Vraćanje JSON odgovora
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Password reset successfully",
	})
}
