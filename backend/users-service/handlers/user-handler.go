package handlers

import (
	"context"
	"encoding/json"
	"fmt" // Ostavljamo ga, ali ga nećemo koristiti za logovanje aplikacije
	"net/http"
	"os"
	"strings"
	"time"
	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/services"
	"trello-project/microservices/users-service/utils"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
)

type UserHandler struct {
	UserService *services.UserService
	JWTService  *services.JWTService
	BlackList   map[string]bool
}

func checkRole(r *http.Request, allowedRoles []string) error {
	userRole := r.Header.Get("Role")
	logging.Logger.Debugf("Event ID: CHECK_ROLE, Description: User Role in Request Header: %s", userRole)

	if userRole == "" {
		logging.Logger.Warn("Event ID: CHECK_ROLE_MISSING, Description: Role is missing in request header.")
		return fmt.Errorf("role is missing in request header")
	}

	// Provera da li je uloga dozvoljena
	for _, role := range allowedRoles {
		if role == userRole {
			logging.Logger.Debugf("Event ID: CHECK_ROLE_ALLOWED, Description: User role '%s' is allowed.", userRole)
			return nil
		}
	}
	logging.Logger.Warnf("Event ID: CHECK_ROLE_FORBIDDEN, Description: Access forbidden: user role '%s' does not have the required role. Allowed roles: %v", userRole, allowedRoles)
	return fmt.Errorf("access forbidden: user does not have the required role")
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: REGISTER_HANDLER_START, Description: Starting Register handler.")
	var requestData struct {
		User         models.User `json:"user"`
		CaptchaToken string      `json:"captchaToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logging.Logger.Warnf("Event ID: REGISTER_INVALID_REQUEST_DATA, Description: Invalid request data for registration: %v", err)
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}

	captchaToken := requestData.CaptchaToken
	if captchaToken == "" {
		logging.Logger.Warn("Event ID: REGISTER_MISSING_CAPTCHA, Description: Missing CAPTCHA token during registration.")
		http.Error(w, "Missing CAPTCHA token", http.StatusBadRequest)
		return
	}

	// Validacija CAPTCHA tokena
	isValid, err := utils.VerifyCaptcha(captchaToken)
	if err != nil {
		logging.Logger.Errorf("Event ID: REGISTER_CAPTCHA_VALIDATION_FAILED, Description: CAPTCHA validation failed: %v", err)
		http.Error(w, fmt.Sprintf("CAPTCHA validation failed: %v", err), http.StatusInternalServerError)
		return
	}
	if !isValid {
		logging.Logger.Warn("Event ID: REGISTER_CAPTCHA_VERIFICATION_FAILED, Description: CAPTCHA verification failed.")
		http.Error(w, "CAPTCHA verification failed", http.StatusForbidden)
		return
	}
	logging.Logger.Debug("Event ID: REGISTER_CAPTCHA_SUCCESS, Description: CAPTCHA successfully verified.")

	user := requestData.User

	if err := h.UserService.ValidatePassword(user.Password); err != nil {
		logging.Logger.Warnf("Event ID: REGISTER_PASSWORD_VALIDATION_FAILED, Description: Password validation failed for username '%s': %v", user.Username, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password is too common. Please choose a stronger one.",
		})

		return
	}
	logging.Logger.Debugf("Event ID: REGISTER_PASSWORD_VALIDATION_SUCCESS, Description: Password validated for username: %s", user.Username)

	// Proveri da li korisničko ime već postoji
	var existingUser models.User
	err = h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&existingUser)
	if err == nil {
		logging.Logger.Warnf("Event ID: REGISTER_USERNAME_EXISTS, Description: Username '%s' already exists.", user.Username)
		// Ako korisnik sa datim korisničkim imenom postoji, vraćamo grešku
		http.Error(w, "Username already exists", http.StatusConflict)
		return
	}
	logging.Logger.Debugf("Event ID: REGISTER_USERNAME_AVAILABLE, Description: Username '%s' is available.", user.Username)

	// Nastavi sa registracijom korisnika
	err = h.UserService.RegisterUser(user)
	if err != nil {
		logging.Logger.Errorf("Event ID: REGISTER_USER_FAILED, Description: Failed to register user '%s': %v", user.Username, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful. Check your email for confirmation link."})
	logging.Logger.Infof("Event ID: REGISTER_SUCCESS, Description: User '%s' registered successfully. Confirmation email sent.", user.Username)
}

// ConfirmEmail kreira korisnika u bazi i redirektuje na login stranicu
func (h *UserHandler) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: CONFIRM_EMAIL_HANDLER_START, Description: Received request for email confirmation.")

	var requestData struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logging.Logger.Warnf("Event ID: CONFIRM_EMAIL_INVALID_REQUEST, Description: Invalid request data for email confirmation: %v", err)
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}
	logging.Logger.Debug("Event ID: CONFIRM_EMAIL_DECODE_SUCCESS, Description: Successfully decoded request data for email confirmation.")

	// Verifikacija tokena
	email, err := h.UserService.JWTService.VerifyEmailVerificationToken(requestData.Token)
	if err != nil {
		logging.Logger.Warnf("Event ID: CONFIRM_EMAIL_TOKEN_INVALID_OR_EXPIRED, Description: Invalid or expired email confirmation token: %v", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Infof("Event ID: CONFIRM_EMAIL_TOKEN_VERIFIED, Description: Token verified for email: %s", email)

	// Proveri da li postoji keširan token
	tokenData, ok := h.UserService.TokenCache[email]
	if !ok {
		logging.Logger.Warnf("Event ID: CONFIRM_EMAIL_TOKEN_NOT_FOUND_CACHE, Description: Token not found in cache for email: %s", email)
		http.Error(w, "Token expired or not found", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: CONFIRM_EMAIL_TOKEN_FOUND_CACHE, Description: Token found in cache for email: %s", email)

	// Parsiranje podataka iz keša
	dataParts := strings.Split(tokenData, "|")
	if len(dataParts) < 6 {
		logging.Logger.Warnf("Event ID: CONFIRM_EMAIL_INVALID_TOKEN_DATA, Description: Invalid token data format in cache for email: %s", email)
		http.Error(w, "Invalid token data", http.StatusBadRequest)
		return
	}
	name := dataParts[1]
	lastName := dataParts[2]
	username := dataParts[3]
	password := dataParts[4]
	role := dataParts[5]
	logging.Logger.Debugf("Event ID: CONFIRM_EMAIL_DATA_PARSED, Description: Parsed data from cache for email '%s': Username: %s, Role: %s", email, username, role)

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
		logging.Logger.Errorf("Event ID: CONFIRM_EMAIL_CREATE_USER_FAILED, Description: Error saving user to database for email '%s': %v", email, err)
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: CONFIRM_EMAIL_USER_SAVED, Description: User '%s' successfully saved to database.", user.Email)

	// Brisanje tokena iz keša nakon uspešne verifikacije
	delete(h.UserService.TokenCache, email)
	logging.Logger.Debugf("Event ID: CONFIRM_EMAIL_TOKEN_DELETED, Description: Token deleted from cache for email: %s", email)

	// Redirektovanje korisnika na login stranicu
	w.Header().Set("Location", "https://localhost:4200/login")
	w.WriteHeader(http.StatusFound)
	logging.Logger.Infof("Event ID: CONFIRM_EMAIL_REDIRECT, Description: User redirected to login page after email confirmation for %s.", user.Email)
}

func (h *UserHandler) VerifyCode(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: VERIFY_CODE_HANDLER_START, Description: Starting VerifyCode handler.")
	var requestData struct {
		Username string `json:"username"`
		Code     string `json:"code"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logging.Logger.Warnf("Event ID: VERIFY_CODE_INVALID_REQUEST, Description: Error decoding request for VerifyCode: %v", err)
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}
	logging.Logger.Infof("Event ID: VERIFY_CODE_REQUEST_RECEIVED, Description: Received verification request for user '%s' with code: %s", requestData.Username, requestData.Code)

	// Provera da li postoji korisnik sa datim username-om
	var user models.User
	err := h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": requestData.Username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: VERIFY_CODE_USER_NOT_FOUND, Description: User not found with username: %s", requestData.Username)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	logging.Logger.Debugf("Event ID: VERIFY_CODE_USER_FOUND, Description: User '%s' found in database.", user.Username)

	// Provera da li se kod poklapa i da li je istekao
	if user.VerificationCode != requestData.Code || time.Now().After(user.VerificationExpiry) {
		logging.Logger.Warnf("Event ID: VERIFY_CODE_INVALID_OR_EXPIRED, Description: Invalid or expired verification code for user '%s'. Provided code: %s, Expected: %s, Expired: %t", requestData.Username, requestData.Code, user.VerificationCode, time.Now().After(user.VerificationExpiry))
		// Brišemo korisnika jer kod nije validan ili je istekao
		_, delErr := h.UserService.UserCollection.DeleteOne(context.Background(), bson.M{"username": requestData.Username})
		if delErr != nil {
			logging.Logger.Errorf("Event ID: VERIFY_CODE_DELETE_FAILED, Description: Error deleting user '%s' due to invalid/expired code: %v", requestData.Username, delErr)
		}
		logging.Logger.Infof("Event ID: VERIFY_CODE_USER_DELETED, Description: User '%s' deleted due to invalid or expired verification code.", requestData.Username)
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
		logging.Logger.Errorf("Event ID: VERIFY_CODE_ACTIVATE_FAILED, Description: Failed to activate user '%s': %v", requestData.Username, err)
		http.Error(w, "Failed to activate user", http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: VERIFY_CODE_SUCCESS, Description: User '%s' successfully activated.", user.Username)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User verified and saved successfully."))
}

func (h *UserHandler) DeleteAccountHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: DELETE_ACCOUNT_HANDLER_START, Description: Starting DeleteAccountHandler.")
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Event ID: DELETE_ACCOUNT_AUTH_FAILED, Description: Access forbidden for DeleteAccountHandler: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		logging.Logger.Warn("Event ID: DELETE_ACCOUNT_MISSING_AUTH_HEADER, Description: Missing Authorization header for account deletion.")
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	claims, err := h.JWTService.ValidateToken(tokenString)
	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_CLAIMS_DECODED, Description: Decoded claims: %+v", claims) // Promenjeno iz fmt.Printf
	if err != nil {
		logging.Logger.Warnf("Event ID: DELETE_ACCOUNT_INVALID_TOKEN, Description: Invalid token for account deletion: %v", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}

	username := claims.Username
	logging.Logger.Infof("Event ID: DELETE_ACCOUNT_REQUEST, Description: Request to delete account for user: %s", username)    // Promenjeno iz fmt.Printf
	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_USERNAME_FROM_TOKEN, Description: Username from token: %s", username)      // Promenjeno iz fmt.Println
	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_TOKEN_TRUNCATED, Description: Token (truncated): %s...", tokenString[:10]) // Promenjeno iz fmt.Println

	err = h.UserService.DeleteAccount(username, tokenString) // Prosledi token dalje
	if err != nil {
		if strings.Contains(err.Error(), "unfinished tasks") {
			logging.Logger.Warnf("Event ID: DELETE_ACCOUNT_UNFINISHED_TASKS, Description: User '%s' cannot be deleted due to unfinished tasks: %v", username, err)
			http.Error(w, err.Error(), http.StatusConflict)
		} else {
			logging.Logger.Errorf("Event ID: DELETE_ACCOUNT_FAILED, Description: Failed to delete account for '%s': %v", username, err)
			http.Error(w, "Failed to delete account: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Account deleted successfully"})
	logging.Logger.Infof("Event ID: DELETE_ACCOUNT_SUCCESS, Description: Account for user '%s' deleted successfully.", username)
}

func (h *UserHandler) GetUserForCurrentSession(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_USER_SESSION_START, Description: Starting GetUserForCurrentSession handler.")
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Event ID: GET_USER_SESSION_AUTH_FAILED, Description: Access forbidden for GetUserForCurrentSession: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		logging.Logger.Warn("Event ID: GET_USER_SESSION_MISSING_AUTH_HEADER, Description: Missing Authorization header for getting user session.")
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	if len(tokenString) > 7 && strings.HasPrefix(tokenString, "Bearer ") {
		tokenString = tokenString[7:]
	}

	claims, err := h.JWTService.ValidateToken(tokenString)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_USER_SESSION_INVALID_TOKEN, Description: Invalid token for getting user session: %v", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: GET_USER_SESSION_TOKEN_VALIDATED, Description: Token validated for user: %s", claims.Username)

	var user models.User
	err = h.UserService.UserCollection.FindOne(context.Background(), bson.M{"username": claims.Username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_USER_SESSION_USER_NOT_FOUND, Description: User '%s' not found in database for current session: %v", claims.Username, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	logging.Logger.Debugf("Event ID: GET_USER_SESSION_USER_FOUND, Description: User '%s' found for current session.", user.Username)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Uklanjanje osetljivih podataka
	user.Password = ""
	user.VerificationCode = ""

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		logging.Logger.Errorf("Event ID: GET_USER_SESSION_ENCODE_FAILED, Description: Failed to encode user data for current session: %v", err)
		http.Error(w, "Failed to encode user data", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: GET_USER_SESSION_SUCCESS, Description: User data sent for current session for user: %s", user.Username)
}

// ChangePassword menja lozinku korisniku
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: CHANGE_PASSWORD_HANDLER_START, Description: Starting ChangePassword handler.")
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_AUTH_FAILED, Description: Access forbidden for ChangePassword: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	var requestData struct {
		OldPassword     string `json:"oldPassword"`
		NewPassword     string `json:"newPassword"`
		ConfirmPassword string `json:"confirmPassword"`
	}

	// Parsiranje podataka iz zahteva
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_INVALID_REQUEST, Description: Invalid request data for ChangePassword: %v", err)
		http.Error(w, "Invalid request data", http.StatusBadRequest)
		return
	}
	logging.Logger.Debug("Event ID: CHANGE_PASSWORD_DECODE_SUCCESS, Description: Successfully decoded request data for password change.")

	// Dohvati token iz Authorization header-a
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		logging.Logger.Warn("Event ID: CHANGE_PASSWORD_MISSING_AUTH_HEADER, Description: Missing Authorization header for password change.")
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
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_INVALID_TOKEN, Description: Invalid token for password change: %v", err)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return
	}
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_TOKEN_VALIDATED, Description: Token validated for user: %s", claims.Username)

	// Provera whitelist i blacklist pravila za novu lozinku
	if err := h.UserService.ValidatePassword(requestData.NewPassword); err != nil {
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_NEW_PASSWORD_VALIDATION_FAILED, Description: New password validation failed for '%s': %v", claims.Username, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password is too common. Please choose a stronger one.",
		})

		return
	}
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_NEW_PASSWORD_VALIDATED, Description: New password validated for user: %s", claims.Username)

	// Pozovi servisnu metodu za promenu lozinke
	err = h.UserService.ChangePassword(claims.Username, requestData.OldPassword, requestData.NewPassword, requestData.ConfirmPassword)
	if err != nil {
		logging.Logger.Errorf("Event ID: CHANGE_PASSWORD_SERVICE_FAILED, Description: Service failed to change password for '%s': %v", claims.Username, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Ako je uspešno, pošaljemo JSON odgovor
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{"message": "Password updated successfully"}
	json.NewEncoder(w).Encode(response)
	logging.Logger.Infof("Event ID: CHANGE_PASSWORD_SUCCESS, Description: Password updated successfully for user: %s", claims.Username)

}

// func (h *UserHandler) GetAllMembers(w http.ResponseWriter, r *http.Request) {
//  // Pravljenje filtera koji selektuje samo korisnike čiji je role = "member"
//  filter := bson.M{"role": "member"}

//  // Izvršavanje upita na bazi
//  cursor, err := h.UserService.UserCollection.Find(context.Background(), filter)
//  if err != nil {
//      http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
//      return
//  }
//  defer cursor.Close(context.Background())

//  // Parsiranje rezultata
//  var members []models.User
//  if err := cursor.All(context.Background(), &members); err != nil {
//      http.Error(w, "Failed to parse members", http.StatusInternalServerError)
//      return
//  }

//  // Uklanjanje lozinki iz odgovora
//  for i := range members {
//      members[i].Password = ""
//  }

//  // Slanje JSON odgovora
//  w.Header().Set("Content-Type", "application/json")
//  w.WriteHeader(http.StatusOK)
//  json.NewEncoder(w).Encode(members)
// }

func (h *UserHandler) GetMemberByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_MEMBER_BY_USERNAME_START, Description: Starting GetMemberByUsernameHandler.")
	// Loguj celu putanju za debug
	path := r.URL.Path
	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_USERNAME_PATH, Description: Full URL Path: %s", path) // Promenjeno iz fmt.Printf

	// Parsiraj username iz putanje
	parts := strings.Split(path, "/")
	if len(parts) < 5 { // Očekujemo da je username na kraju URL-a
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_USERNAME_INVALID_URL, Description: Invalid URL format for GetMemberByUsername: %s", path)
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	username := parts[len(parts)-1]
	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_USERNAME_EXTRACTED, Description: Extracted username manually: %s", username) // Promenjeno iz fmt.Printf

	// Ako je username prazan
	if username == "" {
		logging.Logger.Warn("Event ID: GET_MEMBER_BY_USERNAME_MISSING_PARAM, Description: Username parameter is missing for GetMemberByUsername.")
		http.Error(w, "Username parameter is missing", http.StatusBadRequest)
		return
	}

	// Dohvatanje člana iz baze pomoću username-a
	var member models.User
	err := h.UserService.UserCollection.FindOne(r.Context(), bson.M{"username": username}).Decode(&member)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_USERNAME_NOT_FOUND, Description: User not found in database: %s, error: %v", username, err) // Promenjeno iz fmt.Printf
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_USERNAME_FOUND, Description: User found: %+v", member.Username) // Promenjeno iz fmt.Printf

	// Kreiranje odgovora
	response := struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		LastName string `json:"lastName"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		IsActive bool   `json:"isActive"`
	}{
		ID:       member.ID.Hex(),
		Name:     member.Name,
		LastName: member.LastName,
		Username: member.Username,
		Email:    member.Email,
		Role:     member.Role,
		IsActive: member.IsActive,
	}

	// Slanje JSON odgovora
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	logging.Logger.Infof("Event ID: GET_MEMBER_BY_USERNAME_SUCCESS, Description: Member data sent for username: %s", username)
}

func (h *UserHandler) GetMembersByProjectIDHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_MEMBERS_BY_PROJECT_ID_START, Description: Starting GetMembersByProjectIDHandler.")
	// Preuzmite projectId iz parametara URL-a
	vars := mux.Vars(r)
	projectID := vars["projectId"]
	logging.Logger.Debugf("Event ID: GET_MEMBERS_BY_PROJECT_ID_PARAM, Description: Received projectId: %s", projectID)

	// Kreirajte URL za poziv projects servisa
	projectsServiceURL := os.Getenv("PROJECTS_SERVICE_URL")
	url := fmt.Sprintf("%s/api/projects/%s/members/all", projectsServiceURL, projectID)
	logging.Logger.Debugf("Event ID: GET_MEMBERS_BY_PROJECT_ID_URL, Description: Calling Projects service URL: %s", url)

	// Pošaljite GET zahtev ka projects servisu
	resp, err := http.Get(url)
	if err != nil {
		logging.Logger.Errorf("Event ID: GET_MEMBERS_BY_PROJECT_ID_COMMUNICATION_FAILED, Description: Failed to communicate with projects service: %v", err)
		http.Error(w, "Failed to communicate with projects service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Logger.Errorf("Event ID: GET_MEMBERS_BY_PROJECT_ID_SERVICE_ERROR, Description: Projects service returned status %d for project ID %s.", resp.StatusCode, projectID)
		http.Error(w, fmt.Sprintf("Projects service returned status %d", resp.StatusCode), http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: GET_MEMBERS_BY_PROJECT_ID_SERVICE_SUCCESS, Description: Projects service returned OK for project ID: %s", projectID)

	// Pročitajte i prosledite odgovor klijentu
	var members []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		logging.Logger.Errorf("Event ID: GET_MEMBERS_BY_PROJECT_ID_DECODE_FAILED, Description: Failed to decode response from projects service for project ID %s: %v", projectID, err)
		http.Error(w, "Failed to decode response from projects service", http.StatusInternalServerError)
		return
	}
	logging.Logger.Debugf("Event ID: GET_MEMBERS_BY_PROJECT_ID_DECODE_SUCCESS, Description: Successfully decoded members from projects service for project ID: %s. Count: %d", projectID, len(members))

	// Vratite podatke o članovima
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
	logging.Logger.Infof("Event ID: GET_MEMBERS_BY_PROJECT_ID_SUCCESS, Description: Members for project ID '%s' sent successfully.", projectID)
}

// vraca sve korisnike koji imaju role member
func (h *UserHandler) GetAllMembers(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_ALL_MEMBERS_START, Description: Starting GetAllMembers handler.")
	// Pozovi servis da dobavi članove (users sa role = "member")
	members, err := h.UserService.GetAllMembers()
	if err != nil {
		logging.Logger.Errorf("Event ID: GET_ALL_MEMBERS_FAILED, Description: Failed to fetch all members: %v", err)
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: GET_ALL_MEMBERS_SUCCESS, Description: Successfully fetched %d members.", len(members))

	// Slanje JSON odgovora
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(members)
}

func (h *UserHandler) GetRoleByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_ROLE_BY_USERNAME_START, Description: Starting GetRoleByUsernameHandler.")
	vars := mux.Vars(r)
	username := vars["username"]
	logging.Logger.Debugf("Event ID: GET_ROLE_BY_USERNAME_PARAM, Description: Received username: %s", username)

	if username == "" {
		logging.Logger.Warn("Event ID: GET_ROLE_BY_USERNAME_MISSING_PARAM, Description: Username parameter is missing for GetRoleByUsername.")
		http.Error(w, "Username parameter is missing", http.StatusBadRequest)
		return
	}

	role, err := h.UserService.GetRoleByUsername(username)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_ROLE_BY_USERNAME_NOT_FOUND, Description: User '%s' not found for role retrieval: %v", username, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	logging.Logger.Infof("Event ID: GET_ROLE_BY_USERNAME_SUCCESS, Description: Role '%s' retrieved for username: %s", role, username)

	response := map[string]string{
		"username": username,
		"role":     role,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *UserHandler) GetIDByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_ID_BY_USERNAME_START, Description: Starting GetIDByUsernameHandler.")
	username := mux.Vars(r)["username"]
	logging.Logger.Debugf("Event ID: GET_ID_BY_USERNAME_PARAM, Description: Received username: %s", username)

	id, err := h.UserService.GetIDByUsername(username)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_ID_BY_USERNAME_NOT_FOUND, Description: User '%s' not found for ID retrieval: %v", username, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	logging.Logger.Infof("Event ID: GET_ID_BY_USERNAME_SUCCESS, Description: ID '%s' retrieved for username: %s", id.Hex(), username)

	resp := map[string]string{
		"id": id.Hex(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *UserHandler) GetMemberByIDHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Debug("Event ID: GET_MEMBER_BY_ID_START, Description: Starting GetMemberByIDHandler.")
	vars := mux.Vars(r)
	id := vars["id"]
	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_ID_PARAM, Description: Received ID: %s", id)

	member, err := h.UserService.GetMemberByID(r.Context(), id)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_ID_NOT_FOUND, Description: Member not found with ID '%s': %v", id, err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	logging.Logger.Infof("Event ID: GET_MEMBER_BY_ID_SUCCESS, Description: Member data retrieved for ID: %s", id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(member)
}
