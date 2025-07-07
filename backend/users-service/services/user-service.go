package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/rand"
)

// UserService struktura
type UserService struct {
	UserCollection    *mongo.Collection
	TokenCache        map[string]string
	JWTService        *JWTService
	ProjectCollection *mongo.Collection
	TaskCollection    *mongo.Collection
	BlackList         map[string]bool
	HTTPClient        *http.Client
}

func NewUserService(
	userCollection, projectCollection, taskCollection *mongo.Collection,
	jwtService *JWTService,
	blackList map[string]bool,
	httpClient *http.Client,
) *UserService {
	return &UserService{
		UserCollection:    userCollection,
		TokenCache:        make(map[string]string),
		JWTService:        jwtService,
		ProjectCollection: projectCollection,
		TaskCollection:    taskCollection,
		BlackList:         blackList,
		HTTPClient:        httpClient,
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
	log.Println("[DeleteAccount] Pokrenuto za korisnika:", username)

	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		log.Println("[DeleteAccount] Korisnik nije pronađen u bazi:", err)
		return fmt.Errorf("user not found")
	}
	userID := user.ID.Hex()
	role := user.Role
	log.Println("[DeleteAccount] Uloga korisnika:", role)

	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	projectsServiceURL := os.Getenv("PROJECTS_SERVICE_URL")

	if tasksServiceURL == "" || projectsServiceURL == "" {
		log.Println("[DeleteAccount] Nedostaje env promenljiva TASKS_SERVICE_URL ili PROJECTS_SERVICE_URL")
		return fmt.Errorf("TASKS_SERVICE_URL or PROJECTS_SERVICE_URL not set")
	}

	makeAuthorizedGetRequest := func(url, role string) (*http.Response, error) {
		log.Println("[HTTP GET] Poziv na:", url)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.Println("[HTTP GET] Greska pri kreiranju zahteva:", err)
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Role", role)
		return http.DefaultClient.Do(req)
	}

	if role == "manager" {
		url := fmt.Sprintf("%s/api/projects/username/%s", projectsServiceURL, username)
		resp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			log.Println("[Manager] Greska prilikom GET projekata:", err)
			return fmt.Errorf("failed to fetch projects for manager: %v", err)
		}
		defer resp.Body.Close()

		log.Println("[Manager] Status kod:", resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get projects for manager: %v", resp.Status)
		}

		var projects []models.Project
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			log.Println("[Manager] Neuspešno dekodiranje projekata:", err)
			return fmt.Errorf("failed to decode projects: %v", err)
		}
		log.Printf("[Manager] Pronađeno %d projekata\n", len(projects))

		for _, project := range projects {
			url = fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", tasksServiceURL, project.ID.Hex())
			taskResp, err := makeAuthorizedGetRequest(url, role)
			if err != nil {
				log.Println("[Manager] Greška pri provere zadataka:", err)
				return fmt.Errorf("task service error: %v", err)
			}
			defer taskResp.Body.Close()

			var result struct {
				HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
			}
			if err := json.NewDecoder(taskResp.Body).Decode(&result); err != nil {
				log.Println("[Manager] Greska pri dekodiranju task odgovora:", err)
				return fmt.Errorf("error decoding task check: %v", err)
			}
			if result.HasUnfinishedTasks {
				log.Println("[Manager] Projekat sa nedovršenim zadacima:", project.ID.Hex())
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		patchURL := fmt.Sprintf("%s/api/projects/remove-user/%s?role=manager", strings.TrimRight(projectsServiceURL, "/"), userID)
		log.Println("[Manager] PATCH za uklanjanje managera:", patchURL)
		req, err := http.NewRequest(http.MethodPatch, patchURL, nil)
		if err != nil {
			log.Println("[Manager] Greska pri kreiranju PATCH zahteva:", err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("[Manager] Greska pri PATCH pozivu: err=%v, status=%v\n", err, resp.StatusCode)
			return fmt.Errorf("failed to remove manager from projects")
		}
		defer resp.Body.Close()
		log.Println("[Manager] Manager uspešno uklonjen iz projekata")
	}

	if role == "member" {
		url := fmt.Sprintf("%s/api/projects/user-projects/%s", strings.TrimRight(projectsServiceURL, "/"), username)
		resp, err := makeAuthorizedGetRequest(url, role)
		if err != nil {
			log.Println("[Member] Greska prilikom GET projekata:", err)
			return fmt.Errorf("failed to fetch projects for member: %v", err)
		}
		defer resp.Body.Close()

		log.Println("[Member] Status kod:", resp.StatusCode)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to get projects for member: %v", resp.Status)
		}

		var projects []models.Project
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			log.Println("[Member] Greska prilikom dekodiranja projekata:", err)
			return fmt.Errorf("failed to decode projects: %v", err)
		}
		log.Printf("[Member] Pronađeno %d projekata\n", len(projects))

		for _, project := range projects {
			baseTasksURL := strings.TrimRight(tasksServiceURL, "/")
			url := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", baseTasksURL, project.ID.Hex())
			log.Println("[Check Unfinished Tasks] Poziv na:", url)
			taskResp, err := makeAuthorizedGetRequest(url, role)
			if err != nil {
				log.Println("[Member] Greska pri provere zadataka:", err)
				return fmt.Errorf("task service error: %v", err)
			}
			defer taskResp.Body.Close()

			var result struct {
				HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
			}
			if err := json.NewDecoder(taskResp.Body).Decode(&result); err != nil {
				log.Println("[Member] Greska pri dekodiranju task odgovora:", err)
				return fmt.Errorf("error decoding task check: %v", err)
			}
			if result.HasUnfinishedTasks {
				log.Println("[Member] Projekat sa nedovršenim zadacima:", project.ID.Hex())
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		patchURL := fmt.Sprintf("%s/api/projects/remove-user/%s?role=member", strings.TrimRight(projectsServiceURL, "/"), userID)
		log.Println("[Member] PATCH za uklanjanje člana:", patchURL)
		req, err := http.NewRequest(http.MethodPatch, patchURL, nil)
		if err != nil {
			log.Println("[Member] Greska pri kreiranju PATCH zahteva:", err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("[Member] Greska pri PATCH zahtevu: err=%v, status=%v\n", err, resp.StatusCode)
			return fmt.Errorf("failed to remove member from projects")
		}
		defer resp.Body.Close()

		taskRemoveURL := fmt.Sprintf("%s/api/tasks/remove-user/by-username/%s", strings.TrimRight(tasksServiceURL, "/"), username)
		log.Println("[Member] PATCH za uklanjanje korisnika iz taskova:", taskRemoveURL)
		req, err = http.NewRequest(http.MethodPatch, taskRemoveURL, nil)
		if err != nil {
			log.Println("[Member] Greska pri kreiranju PATCH zahteva za taskove:", err)
			return err
		}
		req.Header.Set("Authorization", "Bearer "+authToken)
		resp, err = http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			log.Printf("[Member] Greska pri uklanjanju usera iz taskova: err=%v, status=%v\n", err, resp.StatusCode)
			return fmt.Errorf("failed to remove user from tasks")
		}
		defer resp.Body.Close()
		log.Println("[Member] Korisnik uklonjen iz taskova")
	}

	log.Println("[DeleteAccount] Brisanje korisnika iz MongoDB...")
	res, err := s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		log.Println("[DeleteAccount] Greska prilikom brisanja:", err)
		return fmt.Errorf("failed to delete user: %v", err)
	}
	log.Println("[DeleteAccount] Obrisano dokumenata:", res.DeletedCount)

	log.Println("[DeleteAccount] Pozivam eksterni servis za čišćenje...")
	req, err := http.NewRequest("POST", "http://external-service/api/cleanup-user", nil)
	if err != nil {
		log.Println("Failed to create HTTP request:", err)
	} else {
		q := req.URL.Query()
		q.Add("username", username)
		req.URL.RawQuery = q.Encode()

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			log.Println("Failed to call external cleanup service:", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				log.Printf("External cleanup service returned status: %d\n", resp.StatusCode)
			} else {
				log.Println("External cleanup service called successfully.")
			}
		}
	}

	log.Println("[DeleteAccount] Korisnik uspešno obrisan:", username)
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
