package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

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
	// Provera da li korisnik već postoji
	var existingUser models.User
	if err := s.UserCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&existingUser); err == nil {
		return fmt.Errorf("user with username already exists")
	}

	// Sanitizacija unosa
	user.Username = html.EscapeString(user.Username)
	user.Name = html.EscapeString(user.Name)
	user.LastName = html.EscapeString(user.LastName)
	user.Email = html.EscapeString(user.Email)

	// Hashiranje lozinke pre nego što se sačuva
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %v", err)
	}
	user.Password = string(hashedPassword)

	// Generisanje verifikacionog koda i podešavanje vremena isteka
	verificationCode := fmt.Sprintf("%06d", rand.Intn(1000000))
	expiryTime := time.Now().Add(1 * time.Minute)

	// Postavljanje verifikacionih informacija u model korisnika
	user.VerificationCode = verificationCode
	user.VerificationExpiry = expiryTime
	user.IsActive = false

	// Čuvanje korisnika u bazi sa statusom `inactive`
	if _, err := s.UserCollection.InsertOne(context.Background(), user); err != nil {
		return fmt.Errorf("failed to save user: %v", err)
	}

	// Slanje verifikacionog email-a sa kodom
	subject := "Your Verification Code"
	body := fmt.Sprintf("Your verification code is %s. Please enter it within 1 minute.", verificationCode)
	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	log.Println("Verifikacioni kod poslat korisniku:", user.Email)
	return nil
}

func (s *UserService) ValidatePassword(password string) error {
	log.Println("Počela validacija lozinke:", password)

	if len(password) < 8 {
		log.Println("Lozinka nije dovoljno dugačka.")
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
		log.Println("Lozinka ne sadrži veliko slovo.")
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
		log.Println("Lozinka ne sadrži broj.")
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
		log.Println("Lozinka ne sadrži specijalni karakter.")
		return fmt.Errorf("password must contain at least one special character")
	}

	if s.BlackList[password] {
		log.Println("Lozinka je na black listi.")
		return fmt.Errorf("password is too common. Please choose a stronger one")
	}

	log.Println("Lozinka je prošla validaciju.")
	return nil
}

// GetUnverifiedUserByEmail pronalazi korisnika u bazi po email adresi
func (s *UserService) GetUnverifiedUserByEmail(email string) (models.User, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		return models.User{}, fmt.Errorf("user not found")
	}
	return user, nil
}

// ConfirmAndSaveUser ažurira korisnika i postavlja `IsActive` na true
func (s *UserService) ConfirmAndSaveUser(user models.User) error {
	// Ažuriraj korisnika da bude aktivan
	filter := bson.M{"email": user.Email}
	update := bson.M{"$set": bson.M{"isActive": true}}

	_, err := s.UserCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to activate user: %v", err)
	}

	return nil
}

// CreateUser čuva korisnika u bazi
func (s *UserService) CreateUser(user models.User) error {
	log.Println("Pokušavam da sačuvam korisnika:", user.Email)

	_, err := s.UserCollection.InsertOne(context.Background(), user)
	if err != nil {
		log.Println("Greška prilikom čuvanja korisnika u MongoDB:", err)
		return err
	}

	log.Println("Korisnik sačuvan u MongoDB:", user.Email)
	return nil
}

func (s *UserService) DeleteAccount(username string, authToken string) error {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	userID := user.ID.Hex()
	role := user.Role

	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	projectsServiceURL := os.Getenv("PROJECTS_SERVICE_URL")
	if tasksServiceURL == "" || projectsServiceURL == "" {
		return fmt.Errorf("TASKS_SERVICE_URL or PROJECTS_SERVICE_URL not set")
	}

	makeAuthorizedGetRequest := func(url, role string) (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Role", role)
		return s.HTTPClient.Do(req)
	}

	getProjectIDs := func(url string, role string) ([]string, error) {
		resp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch projects: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to get projects: %v", resp.Status)
		}

		var projects []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			return nil, fmt.Errorf("failed to decode project IDs: %v", err)
		}

		var projectIDs []string
		for _, p := range projects {
			projectIDs = append(projectIDs, p.ID)
		}
		return projectIDs, nil
	}

	var projectIDs []string
	if role == "manager" {
		url := fmt.Sprintf("%s/api/projects/username/%s", strings.TrimRight(projectsServiceURL, "/"), username)
		projectIDs, err = getProjectIDs(url, role)
		if err != nil {
			return err
		}
	} else if role == "member" {
		url := fmt.Sprintf("%s/api/projects/user-projects/%s", strings.TrimRight(projectsServiceURL, "/"), username)
		projectIDs, err = getProjectIDs(url, role)
		if err != nil {
			return err
		}
	}

	for _, projectID := range projectIDs {
		url := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", strings.TrimRight(tasksServiceURL, "/"), projectID)
		taskResp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			return fmt.Errorf("task service error: %v", err)
		}
		defer taskResp.Body.Close()

		var result struct {
			HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
		}
		if err := json.NewDecoder(taskResp.Body).Decode(&result); err != nil {
			return fmt.Errorf("error decoding task check: %v", err)
		}

		if result.HasUnfinishedTasks {
			return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", projectID)
		}
	}

	if role == "manager" {
		for _, projectID := range projectIDs {
			deleteProjectURL := fmt.Sprintf("%s/api/projects/%s", strings.TrimRight(projectsServiceURL, "/"), projectID)
			req, err := http.NewRequest(http.MethodDelete, deleteProjectURL, nil)
			if err != nil {
				return fmt.Errorf("failed to create request to delete project: %v", err)
			}
			req.Header.Set("Authorization", "Bearer "+authToken)
			req.Header.Set("Role", role)
			resp, err := s.HTTPClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to delete project %s: %v", projectID, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to delete project %s: status %d", projectID, resp.StatusCode)
			}
		}
	}

	patchURL := fmt.Sprintf("%s/api/projects/remove-user/%s?role=%s", strings.TrimRight(projectsServiceURL, "/"), userID, role)
	req, err := http.NewRequest(http.MethodPatch, patchURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+authToken)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove user from projects due to HTTP error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err == nil && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to remove user from projects: status %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	if role == "member" {
		taskRemoveURL := fmt.Sprintf("%s/api/tasks/remove-user/by-username/%s", strings.TrimRight(tasksServiceURL, "/"), username)
		req, err := http.NewRequest(http.MethodPatch, taskRemoveURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = s.HTTPClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to remove user from tasks")
		}
		defer resp.Body.Close()
	}

	res, err := s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}
	if res.DeletedCount == 0 {
		return fmt.Errorf("user was not deleted")
	}

	return nil
}

func (s UserService) LoginUser(username, password string) (models.User, string, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return models.User{}, "", errors.New("user not found")
	}

	// Provera hashirane lozinke
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return models.User{}, "", errors.New("invalid password")
	}

	if !user.IsActive {
		return models.User{}, "", errors.New("user not active")
	}

	token, err := s.JWTService.GenerateAuthToken(user.Username, user.Role)
	if err != nil {
		return models.User{}, "", fmt.Errorf("failed to generate token: %v", err)
	}

	return user, token, nil
}

// DeleteExpiredUnverifiedUsers briše korisnike kojima je istekao rok za verifikaciju i koji nisu aktivni
func (s *UserService) DeleteExpiredUnverifiedUsers() {
	filter := bson.M{
		"isActive": false,
		"verificationExpiry": bson.M{
			"$lt": time.Now(),
		},
	}

	// Brišemo sve korisnike koji odgovaraju uslovima
	result, err := s.UserCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		log.Printf("Greška prilikom brisanja korisnika sa isteklim verifikacionim rokom: %v", err)
	} else {
		log.Printf("Obrisano %d korisnika sa isteklim verifikacionim rokom.", result.DeletedCount)
	}
}

func (s *UserService) GetUserForCurrentSession(ctx context.Context, username string) (models.User, error) {
	var user models.User

	err := s.UserCollection.FindOne(ctx, bson.M{"username": username}).Decode(&user)
	if err != nil {
		return models.User{}, fmt.Errorf("user not found")
	}

	user.Password = ""

	return user, nil
}

// ChangePassword menja lozinku korisniku
func (s *UserService) ChangePassword(username, oldPassword, newPassword, confirmPassword string) error {
	// Proveri da li se nova lozinka poklapa sa potvrdom
	if newPassword != confirmPassword {
		return fmt.Errorf("new password and confirmation do not match")
	}

	// Pronađi korisnika u bazi
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Proveri staru lozinku
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return fmt.Errorf("old password is incorrect")
	}

	// Hashuj novu lozinku
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %v", err)
	}

	// Ažuriraj lozinku u bazi
	_, err = s.UserCollection.UpdateOne(
		context.Background(),
		bson.M{"username": username},
		bson.M{"$set": bson.M{"password": string(hashedNewPassword)}},
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	return nil
}

func (s *UserService) SendPasswordResetLink(username, email string) error {
	// Pronađi korisnika u bazi
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if user.Email != email {
		return fmt.Errorf("email does not match")
	}

	// Generiši token za resetovanje lozinke
	token, err := s.JWTService.GenerateEmailVerificationToken(username)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %v", err)
	}

	// Pošalji email sa linkom za resetovanje
	if err := utils.SendPasswordResetEmail(email, token); err != nil {
		return fmt.Errorf("failed to send password reset email: %v", err)
	}

	return nil
}

func (s *UserService) GetMemberByUsernameHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	fmt.Println("Received username:", username)

	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		fmt.Printf("User not found for username: %s, error: %v\n", username, err)
		http.Error(w, "Member not found", http.StatusNotFound)
		return
	}

	// Sakrij lozinku pre slanja odgovora
	user.Password = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// vraca sve korisnike koji imaju role member
func (s *UserService) GetAllMembers() ([]models.User, error) {
	// Pravljenje filtera koji selektuje samo korisnike čiji je role = "member"
	filter := bson.M{"role": "member"}

	// Izvršavanje upita na bazi
	cursor, err := s.UserCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch members: %v", err)
	}
	defer cursor.Close(context.Background())

	// Parsiranje rezultata
	var members []models.User
	if err := cursor.All(context.Background(), &members); err != nil {
		return nil, fmt.Errorf("failed to parse members: %v", err)
	}

	// Uklanjanje lozinki iz odgovora
	for i := range members {
		members[i].Password = ""
	}

	return members, nil
}

func (s *UserService) GetRoleByUsername(username string) (string, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return "", err
	}
	return user.Role, nil
}

func (s *UserService) GetIDByUsername(username string) (primitive.ObjectID, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("user not found: %v", err)
	}
	return user.ID, nil
}

func (s *UserService) GetMemberByID(ctx context.Context, id string) (models.User, error) {
	userID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.User{}, fmt.Errorf("invalid user ID format")
	}

	var member models.User
	err = s.UserCollection.FindOne(ctx, bson.M{"_id": userID}).Decode(&member)
	if err != nil {
		return models.User{}, fmt.Errorf("user not found")
	}

	return member, nil
}
