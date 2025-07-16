package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"time"

	"trello-project/microservices/users-service/logging" // Dodato za pristup custom loggeru
	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"github.com/gorilla/mux"
	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/rand"
)

// UserService struktura
type UserService struct {
	UserCollection  *mongo.Collection
	TokenCache      map[string]string
	JWTService      *JWTService
	BlackList       map[string]bool
	HTTPClient      *http.Client
	TasksBreaker    *gobreaker.CircuitBreaker
	ProjectsBreaker *gobreaker.CircuitBreaker
}

func NewUserService(
	userCollection *mongo.Collection,
	jwtService *JWTService,
	blackList map[string]bool,
	httpClient *http.Client,
	tasksBreaker *gobreaker.CircuitBreaker,
	projectsBreaker *gobreaker.CircuitBreaker,
) *UserService {
	logging.Logger.Debug("Event ID: NEW_USER_SERVICE_INIT, Description: Initializing UserService.")
	return &UserService{
		UserCollection:  userCollection,
		TokenCache:      make(map[string]string),
		JWTService:      jwtService,
		BlackList:       blackList,
		HTTPClient:      httpClient,
		TasksBreaker:    tasksBreaker,
		ProjectsBreaker: projectsBreaker,
	}
}

// RegisterUser šalje verifikacioni email korisniku i čuva podatke u kešu
func (s *UserService) RegisterUser(user models.User) error {
	logging.Logger.Debugf("Event ID: REGISTER_USER_START, Description: Attempting to register user with username: %s, email: %s", user.Username, user.Email)

	// Provera da li korisnik već postoji
	var existingUser models.User
	if err := s.UserCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&existingUser); err == nil {
		logging.Logger.Warnf("Event ID: REGISTER_USER_ALREADY_EXISTS, Description: User with username '%s' already exists.", user.Username)
		return fmt.Errorf("user with username already exists")
	}
	logging.Logger.Debugf("Event ID: REGISTER_USER_USERNAME_CHECK_PASSED, Description: Username '%s' is available.", user.Username)

	// Sanitizacija unosa
	user.Username = html.EscapeString(user.Username)
	user.Name = html.EscapeString(user.Name)
	user.LastName = html.EscapeString(user.LastName)
	user.Email = html.EscapeString(user.Email)
	logging.Logger.Debug("Event ID: REGISTER_USER_INPUT_SANITIZED, Description: User input sanitized.")

	// Hashiranje lozinke pre nego što se sačuva
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		logging.Logger.Errorf("Event ID: REGISTER_USER_PASSWORD_HASH_FAILED, Description: Failed to hash password for user '%s': %v", user.Username, err)
		return fmt.Errorf("failed to hash password: %v", err)
	}
	user.Password = string(hashedPassword)
	logging.Logger.Debug("Event ID: REGISTER_USER_PASSWORD_HASHED, Description: Password hashed successfully.")

	// Generisanje verifikacionog koda i podešavanje vremena isteka
	verificationCode := fmt.Sprintf("%06d", rand.Intn(1000000))
	expiryTime := time.Now().Add(1 * time.Minute)

	// Postavljanje verifikacionih informacija u model korisnika
	user.VerificationCode = verificationCode
	user.VerificationExpiry = expiryTime
	user.IsActive = false
	logging.Logger.Debugf("Event ID: REGISTER_USER_VERIFICATION_SET, Description: Verification code '%s' set for user '%s' with expiry: %s", verificationCode, user.Username, expiryTime.String())

	// Čuvanje korisnika u bazi sa statusom `inactive`
	if _, err := s.UserCollection.InsertOne(context.Background(), user); err != nil {
		logging.Logger.Errorf("Event ID: REGISTER_USER_DB_INSERT_FAILED, Description: Failed to save inactive user '%s' to database: %v", user.Username, err)
		return fmt.Errorf("failed to save user: %v", err)
	}
	logging.Logger.Infof("Event ID: REGISTER_USER_DB_INSERT_SUCCESS, Description: Inactive user '%s' saved to database.", user.Username)

	// Slanje verifikacionog email-a sa kodom
	subject := "Your Verification Code"
	body := fmt.Sprintf("Your verification code is %s. Please enter it within 1 minute.", verificationCode)
	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		logging.Logger.Errorf("Event ID: REGISTER_USER_EMAIL_SEND_FAILED, Description: Failed to send verification email to '%s': %v", user.Email, err)
		return fmt.Errorf("failed to send email: %v", err)
	}

	logging.Logger.Infof("Event ID: REGISTER_USER_EMAIL_SENT, Description: Verification code sent to user: %s", user.Email)
	return nil
}

func (s *UserService) ValidatePassword(password string) error {
	logging.Logger.Debug("Event ID: VALIDATE_PASSWORD_START, Description: Starting password validation.")

	if len(password) < 8 {
		logging.Logger.Warn("Event ID: VALIDATE_PASSWORD_TOO_SHORT, Description: Password is too short (less than 8 characters).")
		return fmt.Errorf("password must be at least 8 characters long")
	}

	hasUppercase := false
	for _, char := range password {
		if char >= 'A' && char <= 'Z' {
			hasUppercase = true
			break
		}
	}
	if !hasUppercase {
		logging.Logger.Warn("Event ID: VALIDATE_PASSWORD_NO_UPPERCASE, Description: Password does not contain an uppercase letter.")
		return fmt.Errorf("password must contain at least one uppercase letter")
	}

	hasDigit := false
	for _, char := range password {
		if char >= '0' && char <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		logging.Logger.Warn("Event ID: VALIDATE_PASSWORD_NO_DIGIT, Description: Password does not contain a number.")
		return fmt.Errorf("password must contain at least one number")
	}

	specialChars := "!@#$%^&*.,"
	hasSpecial := false
	for _, char := range password {
		if strings.ContainsRune(specialChars, char) {
			hasSpecial = true
			break
		}
	}
	if !hasSpecial {
		logging.Logger.Warn("Event ID: VALIDATE_PASSWORD_NO_SPECIAL_CHAR, Description: Password does not contain a special character.")
		return fmt.Errorf("password must contain at least one special character")
	}

	if s.BlackList[password] {
		logging.Logger.Warn("Event ID: VALIDATE_PASSWORD_BLACKLISTED, Description: Password is on the blacklist.")
		return fmt.Errorf("password is too common. Please choose a stronger one")
	}

	logging.Logger.Debug("Event ID: VALIDATE_PASSWORD_SUCCESS, Description: Password validation successful.")
	return nil
}

// GetUnverifiedUserByEmail pronalazi korisnika u bazi po email adresi
func (s *UserService) GetUnverifiedUserByEmail(email string) (models.User, error) {
	logging.Logger.Debugf("Event ID: GET_UNVERIFIED_USER_BY_EMAIL_START, Description: Searching for unverified user with email: %s", email)
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": email, "isActive": false}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_UNVERIFIED_USER_BY_EMAIL_NOT_FOUND, Description: Unverified user with email '%s' not found: %v", email, err)
		return models.User{}, fmt.Errorf("user not found")
	}
	logging.Logger.Infof("Event ID: GET_UNVERIFIED_USER_BY_EMAIL_SUCCESS, Description: Found unverified user with email: %s", email)
	return user, nil
}

// ConfirmAndSaveUser ažurira korisnika i postavlja `IsActive` na true
func (s *UserService) ConfirmAndSaveUser(user models.User) error {
	logging.Logger.Debugf("Event ID: CONFIRM_AND_SAVE_USER_START, Description: Attempting to confirm and activate user with email: %s", user.Email)
	// Ažuriraj korisnika da bude aktivan
	filter := bson.M{"email": user.Email}
	update := bson.M{"$set": bson.M{"isActive": true, "verificationCode": "", "verificationExpiry": time.Time{}}} // Reset verification fields

	_, err := s.UserCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		logging.Logger.Errorf("Event ID: CONFIRM_AND_SAVE_USER_ACTIVATION_FAILED, Description: Failed to activate user '%s': %v", user.Email, err)
		return fmt.Errorf("failed to activate user: %v", err)
	}

	logging.Logger.Infof("Event ID: CONFIRM_AND_SAVE_USER_SUCCESS, Description: User '%s' successfully activated.", user.Email)
	return nil
}

// CreateUser čuva korisnika u bazi
func (s *UserService) CreateUser(user models.User) error {
	logging.Logger.Debugf("Event ID: CREATE_USER_START, Description: Attempting to save user to MongoDB: %s", user.Email)

	_, err := s.UserCollection.InsertOne(context.Background(), user)
	if err != nil {
		logging.Logger.Errorf("Event ID: CREATE_USER_DB_INSERT_FAILED, Description: Error saving user to MongoDB '%s': %v", user.Email, err)
		return err
	}

	logging.Logger.Infof("Event ID: CREATE_USER_DB_INSERT_SUCCESS, Description: User saved to MongoDB: %s", user.Email)
	return nil
}

func (s *UserService) DeleteAccount(username string, authToken string) error {
	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_START, Description: Initiating account deletion for user: %s", username)

	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: DELETE_ACCOUNT_USER_NOT_FOUND, Description: User '%s' not found in database: %v", username, err)
		return fmt.Errorf("user not found")
	}
	userID := user.ID.Hex()
	role := user.Role
	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_USER_ROLE, Description: User '%s' has role: %s", username, role)

	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	projectsServiceURL := os.Getenv("PROJECTS_SERVICE_URL")

	if tasksServiceURL == "" || projectsServiceURL == "" {
		logging.Logger.Errorf("Event ID: DELETE_ACCOUNT_ENV_MISSING, Description: TASKS_SERVICE_URL or PROJECTS_SERVICE_URL environment variable not set.")
		return fmt.Errorf("TASKS_SERVICE_URL or PROJECTS_SERVICE_URL not set")
	}

	makeAuthorizedGetRequest := func(url, role string) (*http.Response, error) {
		logging.Logger.Debugf("Event ID: HTTP_GET_REQUEST, Description: Making HTTP GET request to: %s", url)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			logging.Logger.Errorf("Event ID: HTTP_GET_REQUEST_CREATE_FAILED, Description: Error creating HTTP GET request for '%s': %v", url, err)
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Role", role)
		return s.HTTPClient.Do(req)
	}

	if role == "manager" {
		url := fmt.Sprintf("%s/api/projects/username/%s", strings.TrimRight(projectsServiceURL, "/"), username)
		resp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			logging.Logger.Errorf("Event ID: MANAGER_GET_PROJECTS_FAILED, Description: Error fetching projects for manager '%s': %v", username, err)
			return fmt.Errorf("failed to fetch projects for manager: %v", err)
		}
		defer resp.Body.Close()

		logging.Logger.Debugf("Event ID: MANAGER_GET_PROJECTS_STATUS, Description: Projects GET request status code for manager '%s': %d", username, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			logging.Logger.Warnf("Event ID: MANAGER_GET_PROJECTS_NON_OK_STATUS, Description: Non-OK status code (%d) when getting projects for manager '%s'.", resp.StatusCode, username)
			return fmt.Errorf("failed to get projects for manager: %v", resp.Status)
		}

		var projects []models.Project
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			logging.Logger.Errorf("Event ID: MANAGER_DECODE_PROJECTS_FAILED, Description: Failed to decode projects for manager '%s': %v", username, err)
			return fmt.Errorf("failed to decode projects: %v", err)
		}
		logging.Logger.Infof("Event ID: MANAGER_PROJECTS_FOUND, Description: Found %d projects for manager '%s'.", len(projects), username)

		for _, project := range projects {
			url = fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", strings.TrimRight(tasksServiceURL, "/"), project.ID.Hex())
			taskResp, err := makeAuthorizedGetRequest(url, role)
			if err != nil {
				logging.Logger.Errorf("Event ID: MANAGER_TASK_CHECK_FAILED, Description: Error checking for unfinished tasks for project '%s' (manager '%s'): %v", project.ID.Hex(), username, err)
				return fmt.Errorf("task service error: %v", err)
			}
			defer taskResp.Body.Close()

			var result struct {
				HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
			}
			if err := json.NewDecoder(taskResp.Body).Decode(&result); err != nil {
				logging.Logger.Errorf("Event ID: MANAGER_TASK_DECODE_FAILED, Description: Error decoding task check response for project '%s' (manager '%s'): %v", project.ID.Hex(), username, err)
				return fmt.Errorf("error decoding task check: %v", err)
			}
			if result.HasUnfinishedTasks {
				logging.Logger.Warnf("Event ID: MANAGER_PROJECT_HAS_UNFINISHED_TASKS, Description: Project '%s' has unfinished tasks. Cannot delete manager account '%s'.", project.ID.Hex(), username)
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		patchURL := fmt.Sprintf("%s/api/projects/remove-user/%s?role=manager", strings.TrimRight(projectsServiceURL, "/"), userID)
		logging.Logger.Debugf("Event ID: MANAGER_REMOVE_PATCH_REQUEST, Description: Sending PATCH request to remove manager '%s' from projects: %s", username, patchURL)
		req, err := http.NewRequest(http.MethodPatch, patchURL, nil)
		if err != nil {
			logging.Logger.Errorf("Event ID: MANAGER_REMOVE_PATCH_CREATE_FAILED, Description: Error creating PATCH request to remove manager '%s': %v", username, err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = s.HTTPClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("Event ID: MANAGER_REMOVE_PATCH_FAILED, Description: Error in PATCH call to remove manager '%s' from projects: err=%v, status=%v", username, err, resp.StatusCode)
			return fmt.Errorf("failed to remove manager from projects")
		}
		defer resp.Body.Close()
		logging.Logger.Infof("Event ID: MANAGER_REMOVED_FROM_PROJECTS, Description: Manager '%s' successfully removed from projects.", username)
	}

	if role == "member" {
		url := fmt.Sprintf("%s/api/projects/user-projects/%s", strings.TrimRight(projectsServiceURL, "/"), username)
		resp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			logging.Logger.Errorf("Event ID: MEMBER_GET_PROJECTS_FAILED, Description: Error fetching projects for member '%s': %v", username, err)
			return fmt.Errorf("failed to fetch projects for member: %v", err)
		}
		defer resp.Body.Close()

		logging.Logger.Debugf("Event ID: MEMBER_GET_PROJECTS_STATUS, Description: Projects GET request status code for member '%s': %d", username, resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			logging.Logger.Warnf("Event ID: MEMBER_GET_PROJECTS_NON_OK_STATUS, Description: Non-OK status code (%d) when getting projects for member '%s'.", resp.StatusCode, username)
			return fmt.Errorf("failed to get projects for member: %v", resp.Status)
		}

		var projects []models.Project
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			logging.Logger.Errorf("Event ID: MEMBER_DECODE_PROJECTS_FAILED, Description: Error decoding projects for member '%s': %v", username, err)
			return fmt.Errorf("failed to decode projects: %v", err)
		}
		logging.Logger.Infof("Event ID: MEMBER_PROJECTS_FOUND, Description: Found %d projects for member '%s'.", len(projects), username)

		for _, project := range projects {
			url := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", strings.TrimRight(tasksServiceURL, "/"), project.ID.Hex())
			logging.Logger.Debugf("Event ID: MEMBER_CHECK_UNFINISHED_TASKS, Description: Checking unfinished tasks for project '%s' (member '%s') via: %s", project.ID.Hex(), username, url)
			taskResp, err := makeAuthorizedGetRequest(url, role)
			if err != nil {
				logging.Logger.Errorf("Event ID: MEMBER_TASK_CHECK_FAILED, Description: Error checking for unfinished tasks for project '%s' (member '%s'): %v", project.ID.Hex(), username, err)
				return fmt.Errorf("task service error: %v", err)
			}
			defer taskResp.Body.Close()

			var result struct {
				HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
			}
			if err := json.NewDecoder(taskResp.Body).Decode(&result); err != nil {
				logging.Logger.Errorf("Event ID: MEMBER_TASK_DECODE_FAILED, Description: Error decoding task check response for project '%s' (member '%s'): %v", project.ID.Hex(), username, err)
				return fmt.Errorf("error decoding task check: %v", err)
			}
			if result.HasUnfinishedTasks {
				logging.Logger.Warnf("Event ID: MEMBER_PROJECT_HAS_UNFINISHED_TASKS, Description: Project '%s' has unfinished tasks. Cannot delete member account '%s'.", project.ID.Hex(), username)
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		patchURL := fmt.Sprintf("%s/api/projects/remove-user/%s?role=member", strings.TrimRight(projectsServiceURL, "/"), userID)
		logging.Logger.Debugf("Event ID: MEMBER_REMOVE_PROJECTS_PATCH, Description: Sending PATCH request to remove member '%s' from projects: %s", username, patchURL)
		req, err := http.NewRequest(http.MethodPatch, patchURL, nil)
		if err != nil {
			logging.Logger.Errorf("Event ID: MEMBER_REMOVE_PROJECTS_PATCH_CREATE_FAILED, Description: Error creating PATCH request to remove member '%s' from projects: %v", username, err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = s.HTTPClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("Event ID: MEMBER_REMOVE_PROJECTS_PATCH_FAILED, Description: Error in PATCH request to remove member '%s' from projects: err=%v, status=%v", username, err, resp.StatusCode)
			return fmt.Errorf("failed to remove member from projects")
		}
		defer resp.Body.Close()
		logging.Logger.Infof("Event ID: MEMBER_REMOVED_FROM_PROJECTS, Description: Member '%s' successfully removed from projects.", username)

		taskRemoveURL := fmt.Sprintf("%s/api/tasks/remove-user/by-username/%s", strings.TrimRight(tasksServiceURL, "/"), username)
		logging.Logger.Debugf("Event ID: MEMBER_REMOVE_TASKS_PATCH, Description: Sending PATCH request to remove user '%s' from tasks: %s", username, taskRemoveURL)
		req, err = http.NewRequest(http.MethodPatch, taskRemoveURL, nil)
		if err != nil {
			logging.Logger.Errorf("Event ID: MEMBER_REMOVE_TASKS_PATCH_CREATE_FAILED, Description: Error creating PATCH request to remove user '%s' from tasks: %v", username, err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = s.HTTPClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("Event ID: MEMBER_REMOVE_TASKS_PATCH_FAILED, Description: Error removing user '%s' from tasks: err=%v, status=%v", username, err, resp.StatusCode)
			return fmt.Errorf("failed to remove user from tasks")
		}
		defer resp.Body.Close()
		logging.Logger.Infof("Event ID: MEMBER_REMOVED_FROM_TASKS, Description: User '%s' successfully removed from tasks.", username)
	}

	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_MONGO_DELETE_START, Description: Deleting user '%s' from MongoDB...", username)
	res, err := s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		logging.Logger.Errorf("Event ID: DELETE_ACCOUNT_MONGO_DELETE_FAILED, Description: Error deleting user '%s' from MongoDB: %v", username, err)
		return fmt.Errorf("failed to delete user: %v", err)
	}
	logging.Logger.Infof("Event ID: DELETE_ACCOUNT_MONGO_DELETE_SUCCESS, Description: Deleted %d documents for user '%s' from MongoDB.", res.DeletedCount, username)

	logging.Logger.Debugf("Event ID: DELETE_ACCOUNT_EXTERNAL_CLEANUP_CALL_START, Description: Calling external service for cleanup for user: %s", username)
	req, err := http.NewRequest("POST", "http://external-service/api/cleanup-user", nil)
	if err != nil {
		logging.Logger.Errorf("Event ID: EXTERNAL_CLEANUP_REQUEST_CREATE_FAILED, Description: Failed to create HTTP request for external cleanup service for user '%s': %v", username, err)
	} else {
		q := req.URL.Query()
		q.Add("username", username)
		req.URL.RawQuery = q.Encode()

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			logging.Logger.Errorf("Event ID: EXTERNAL_CLEANUP_SERVICE_CALL_FAILED, Description: Failed to call external cleanup service for user '%s': %v", username, err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logging.Logger.Warnf("Event ID: EXTERNAL_CLEANUP_SERVICE_NON_OK_STATUS, Description: External cleanup service returned status: %d for user '%s'.", resp.StatusCode, username)
			} else {
				logging.Logger.Infof("Event ID: EXTERNAL_CLEANUP_SERVICE_CALLED_SUCCESS, Description: External cleanup service called successfully for user: %s", username)
			}
		}
	}

	logging.Logger.Infof("Event ID: DELETE_ACCOUNT_COMPLETED, Description: User '%s' account deletion process completed.", username)
	return nil
}

func (s UserService) LoginUser(username, password string) (models.User, string, error) {
	logging.Logger.Debugf("Event ID: LOGIN_USER_START, Description: Attempting to log in user: %s", username)
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: LOGIN_USER_NOT_FOUND, Description: User '%s' not found during login: %v", username, err)
		return models.User{}, "", errors.New("user not found")
	}
	logging.Logger.Debugf("Event ID: LOGIN_USER_FOUND, Description: User '%s' found. Proceeding with password comparison.", username)

	// Provera hashirane lozinke
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		logging.Logger.Warnf("Event ID: LOGIN_USER_INVALID_PASSWORD, Description: Invalid password provided for user: %s", username)
		return models.User{}, "", errors.New("invalid password")
	}
	logging.Logger.Debugf("Event ID: LOGIN_USER_PASSWORD_MATCH, Description: Password matched for user: %s", username)

	if !user.IsActive {
		logging.Logger.Warnf("Event ID: LOGIN_USER_INACTIVE, Description: Attempted login for inactive user: %s", username)
		return models.User{}, "", errors.New("user not active")
	}
	logging.Logger.Debugf("Event ID: LOGIN_USER_ACTIVE, Description: User '%s' is active. Generating auth token.", username)

	token, err := s.JWTService.GenerateAuthToken(user.Username, user.Role)
	if err != nil {
		logging.Logger.Errorf("Event ID: LOGIN_USER_TOKEN_GENERATION_FAILED, Description: Failed to generate auth token for user '%s': %v", user.Username, err)
		return models.User{}, "", fmt.Errorf("failed to generate token: %v", err)
	}
	logging.Logger.Infof("Event ID: LOGIN_USER_SUCCESS, Description: Successfully logged in user '%s' and generated token.", username)

	return user, token, nil
}

// DeleteExpiredUnverifiedUsers briše korisnike kojima je istekao rok za verifikaciju i koji nisu aktivni
func (s *UserService) DeleteExpiredUnverifiedUsers() {
	logging.Logger.Debug("Event ID: DELETE_EXPIRED_UNVERIFIED_USERS_START, Description: Starting periodic cleanup of expired unverified users.")
	filter := bson.M{
		"isActive": false,
		"verificationExpiry": bson.M{
			"$lt": time.Now(),
		},
	}

	// Brišemo sve korisnike koji odgovaraju uslovima
	result, err := s.UserCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Event ID: DELETE_EXPIRED_UNVERIFIED_USERS_FAILED, Description: Error deleting users with expired verification: %v", err)
	} else {
		logging.Logger.Infof("Event ID: DELETE_EXPIRED_UNVERIFIED_USERS_SUCCESS, Description: Deleted %d users with expired verification.", result.DeletedCount)
	}
}

func (s *UserService) GetUserForCurrentSession(ctx context.Context, username string) (models.User, error) {
	logging.Logger.Debugf("Event ID: GET_USER_FOR_CURRENT_SESSION_START, Description: Fetching user '%s' for current session.", username)
	var user models.User

	err := s.UserCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_USER_FOR_CURRENT_SESSION_NOT_FOUND, Description: User '%s' not found for current session: %v", username, err)
		return models.User{}, fmt.Errorf("user not found")
	}

	user.Password = "" // Ensure password is not exposed
	logging.Logger.Infof("Event ID: GET_USER_FOR_CURRENT_SESSION_SUCCESS, Description: User '%s' fetched for current session. Password redacted.", username)
	return user, nil
}

// ChangePassword menja lozinku korisniku
func (s *UserService) ChangePassword(username, oldPassword, newPassword, confirmPassword string) error {
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_START, Description: Attempting to change password for user: %s", username)
	// Proveri da li se nova lozinka poklapa sa potvrdom
	if newPassword != confirmPassword {
		logging.Logger.Warn("Event ID: CHANGE_PASSWORD_MISMATCH, Description: New password and confirmation do not match.")
		return fmt.Errorf("new password and confirmation do not match")
	}

	// Pronađi korisnika u bazi
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_USER_NOT_FOUND, Description: User '%s' not found during password change: %v", username, err)
		return fmt.Errorf("user not found")
	}
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_USER_FOUND, Description: User '%s' found. Verifying old password.", username)

	// Proveri staru lozinku
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		logging.Logger.Warnf("Event ID: CHANGE_PASSWORD_OLD_PASSWORD_INCORRECT, Description: Incorrect old password provided for user: %s", username)
		return fmt.Errorf("old password is incorrect")
	}
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_OLD_PASSWORD_CORRECT, Description: Old password is correct for user: %s", username)

	// Hashuj novu lozinku
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		logging.Logger.Errorf("Event ID: CHANGE_PASSWORD_HASH_FAILED, Description: Failed to hash new password for user '%s': %v", username, err)
		return fmt.Errorf("failed to hash new password: %v", err)
	}
	logging.Logger.Debugf("Event ID: CHANGE_PASSWORD_NEW_PASSWORD_HASHED, Description: New password hashed for user: %s", username)

	// Ažuriraj lozinku u bazi
	_, err = s.UserCollection.UpdateOne(
		context.Background(),
		bson.M{"username": username},
		bson.M{"$set": bson.M{"password": string(hashedNewPassword)}},
	)
	if err != nil {
		logging.Logger.Errorf("Event ID: CHANGE_PASSWORD_DB_UPDATE_FAILED, Description: Failed to update password for user '%s' in database: %v", username, err)
		return fmt.Errorf("failed to update password: %v", err)
	}

	logging.Logger.Infof("Event ID: CHANGE_PASSWORD_SUCCESS, Description: Password successfully changed for user: %s", username)
	return nil
}

func (s *UserService) SendPasswordResetLink(username, email string) error {
	logging.Logger.Debugf("Event ID: SEND_PASSWORD_RESET_LINK_START, Description: Attempting to send password reset link for user '%s' to email: %s", username, email)
	// Pronađi korisnika u bazi
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: SEND_PASSWORD_RESET_LINK_USER_NOT_FOUND, Description: User '%s' not found when sending reset link: %v", username, err)
		return fmt.Errorf("user not found")
	}
	logging.Logger.Debugf("Event ID: SEND_PASSWORD_RESET_LINK_USER_FOUND, Description: User '%s' found. Verifying email match.", username)

	if user.Email != email {
		logging.Logger.Warnf("Event ID: SEND_PASSWORD_RESET_LINK_EMAIL_MISMATCH, Description: Provided email '%s' does not match user's email '%s' for user '%s'.", email, user.Email, username)
		return fmt.Errorf("email does not match")
	}
	logging.Logger.Debugf("Event ID: SEND_PASSWORD_RESET_LINK_EMAIL_MATCH, Description: Email matched for user '%s'. Generating reset token.", username)

	// Generiši token za resetovanje lozinke
	token, err := s.JWTService.GenerateEmailVerificationToken(username) // Reusing verification token logic, consider a dedicated reset token
	if err != nil {
		logging.Logger.Errorf("Event ID: SEND_PASSWORD_RESET_LINK_TOKEN_GENERATION_FAILED, Description: Failed to generate reset token for user '%s': %v", username, err)
		return fmt.Errorf("failed to generate reset token: %v", err)
	}
	logging.Logger.Debugf("Event ID: SEND_PASSWORD_RESET_LINK_TOKEN_GENERATED, Description: Reset token generated for user: %s", username)

	// Pošalji email sa linkom za resetovanje
	if err := utils.SendPasswordResetEmail(email, token); err != nil {
		logging.Logger.Errorf("Event ID: SEND_PASSWORD_RESET_LINK_EMAIL_SEND_FAILED, Description: Failed to send password reset email to '%s' for user '%s': %v", email, username, err)
		return fmt.Errorf("failed to send password reset email: %v", err)
	}

	logging.Logger.Infof("Event ID: SEND_PASSWORD_RESET_LINK_SUCCESS, Description: Password reset link successfully sent to '%s' for user: %s", email, username)
	return nil
}

func (s *UserService) GetMemberByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_USERNAME_HANDLER_START, Description: Received request for username: %s", username)

	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_USERNAME_HANDLER_NOT_FOUND, Description: User not found for username: %s, error: %v", username, err)
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	// Sakrij lozinku pre slanja odgovora
	user.Password = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
	logging.Logger.Infof("Event ID: GET_MEMBER_BY_USERNAME_HANDLER_SUCCESS, Description: Successfully retrieved and sent user details for username: %s", username)
}

// vraca sve korisnike koji imaju role member
func (s *UserService) GetAllMembers() ([]models.User, error) {
	logging.Logger.Debug("Event ID: GET_ALL_MEMBERS_START, Description: Attempting to retrieve all users with role 'member'.")
	// Pravljenje filtera koji selektuje samo korisnike čiji je role = "member"
	filter := bson.M{"role": "member"}

	// Izvršavanje upita na bazi
	cursor, err := s.UserCollection.Find(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Event ID: GET_ALL_MEMBERS_DB_QUERY_FAILED, Description: Failed to fetch members from database: %v", err)
		return nil, fmt.Errorf("failed to fetch members: %v", err)
	}
	defer cursor.Close(context.Background())

	// Parsiranje rezultata
	var members []models.User
	if err := cursor.All(context.Background(), &members); err != nil {
		logging.Logger.Errorf("Event ID: GET_ALL_MEMBERS_DECODE_FAILED, Description: Failed to parse members from database cursor: %v", err)
		return nil, fmt.Errorf("failed to parse members: %v", err)
	}

	// Uklanjanje lozinki iz odgovora
	for i := range members {
		members[i].Password = ""
	}

	logging.Logger.Infof("Event ID: GET_ALL_MEMBERS_SUCCESS, Description: Successfully retrieved %d members.", len(members))
	return members, nil
}

func (s *UserService) GetRoleByUsername(username string) (string, error) {
	logging.Logger.Debugf("Event ID: GET_ROLE_BY_USERNAME_START, Description: Attempting to retrieve role for username: %s", username)
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_ROLE_BY_USERNAME_NOT_FOUND, Description: User '%s' not found when getting role: %v", username, err)
		return "", err
	}
	logging.Logger.Infof("Event ID: GET_ROLE_BY_USERNAME_SUCCESS, Description: Successfully retrieved role '%s' for username: %s", user.Role, username)
	return user.Role, nil
}

func (s *UserService) GetIDByUsername(username string) (primitive.ObjectID, error) {
	logging.Logger.Debugf("Event ID: GET_ID_BY_USERNAME_START, Description: Attempting to retrieve ID for username: %s", username)
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_ID_BY_USERNAME_NOT_FOUND, Description: User '%s' not found when getting ID: %v", username, err)
		return primitive.NilObjectID, fmt.Errorf("user not found: %v", err)
	}
	logging.Logger.Infof("Event ID: GET_ID_BY_USERNAME_SUCCESS, Description: Successfully retrieved ID '%s' for username: %s", user.ID.Hex(), username)
	return user.ID, nil
}

func (s *UserService) GetMemberByID(ctx context.Context, id string) (models.User, error) {
	logging.Logger.Debugf("Event ID: GET_MEMBER_BY_ID_START, Description: Attempting to retrieve member by ID: %s", id)
	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_ID_INVALID_FORMAT, Description: Invalid user ID format: %s, error: %v", id, err)
		return models.User{}, fmt.Errorf("invalid user ID format")
	}

	var member models.User
	err = s.UserCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&member)
	if err != nil {
		logging.Logger.Warnf("Event ID: GET_MEMBER_BY_ID_NOT_FOUND, Description: User with ID '%s' not found: %v", id, err)
		return models.User{}, fmt.Errorf("user not found")
	}
	logging.Logger.Infof("Event ID: GET_MEMBER_BY_ID_SUCCESS, Description: Successfully retrieved member with ID: %s", id)
	return member, nil
}
