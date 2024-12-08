package services

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"

	"time"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
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
}

func NewUserService(userCollection, projectCollection, taskCollection *mongo.Collection, jwtService *JWTService, blackList map[string]bool) *UserService {
	return &UserService{

		UserCollection:    userCollection,
		TokenCache:        make(map[string]string),
		JWTService:        &JWTService{},
		ProjectCollection: projectCollection,
		TaskCollection:    taskCollection,
		BlackList:         blackList,
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

func (s *UserService) DeleteAccount(username string) error {
	// Pronađi korisnika po username
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found")
	}
	userID := user.ID

	// Provera uloge korisnika
	if user.Role == "manager" {
		// Pronađi projekte koje je menadžer kreirao
		projectFilter := bson.M{"manager_id": userID}
		cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
		if err != nil {
			return fmt.Errorf("failed to fetch projects for manager: %v", err)
		}
		defer cursor.Close(context.Background())

		// Proveri da li projekti imaju nezavršene zadatke
		for cursor.Next(context.Background()) {
			var project models.Project
			if err := cursor.Decode(&project); err != nil {
				return fmt.Errorf("failed to decode project: %v", err)
			}

			taskFilter := bson.M{
				"projectId": project.ID.Hex(),
				"status":    bson.M{"$ne": "Completed"},
			}
			count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
			if err != nil {
				return fmt.Errorf("failed to check tasks for project '%s': %v", project.ID.Hex(), err)
			}

			if count > 0 {
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		// Ukloni menadžera iz projekata, ali ostavi projekte
		update := bson.M{"$unset": bson.M{"manager_id": ""}}
		_, err = s.ProjectCollection.UpdateMany(context.Background(), projectFilter, update)
		if err != nil {
			return fmt.Errorf("failed to remove manager from projects: %v", err)
		}
	}

	// Član: proveri da li je deo projekata sa nezavršenim zadacima
	if user.Role == "member" {
		projectFilter := bson.M{"members._id": userID}
		cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
		if err != nil {
			return fmt.Errorf("failed to fetch projects for member: %v", err)
		}
		defer cursor.Close(context.Background())

		for cursor.Next(context.Background()) {
			var project models.Project
			if err := cursor.Decode(&project); err != nil {
				return fmt.Errorf("failed to decode project: %v", err)
			}

			taskFilter := bson.M{
				"projectId": project.ID.Hex(),
				"status":    bson.M{"$ne": "Completed"},
			}
			count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
			if err != nil {
				return fmt.Errorf("failed to check tasks for project '%s': %v", project.ID.Hex(), err)
			}

			if count > 0 {
				return fmt.Errorf("cannot delete account: project '%s' has unfinished tasks", project.ID.Hex())
			}
		}

		// Ukloni člana iz projekata
		projectUpdateFilter := bson.M{"members._id": userID}
		projectUpdate := bson.M{"$pull": bson.M{"members": bson.M{"_id": userID}}}
		_, err = s.ProjectCollection.UpdateMany(context.Background(), projectUpdateFilter, projectUpdate)
		if err != nil {
			return fmt.Errorf("failed to remove user from projects: %v", err)
		}

		// Ukloni člana iz zadataka - `assignees` polje
		taskAssigneesUpdateFilter := bson.M{"assignees": userID}
		taskAssigneesUpdate := bson.M{"$pull": bson.M{"assignees": userID}}
		_, err = s.TaskCollection.UpdateMany(context.Background(), taskAssigneesUpdateFilter, taskAssigneesUpdate)
		if err != nil {
			return fmt.Errorf("failed to remove user from task assignees: %v", err)
		}

		// Ukloni člana iz zadataka - `members` polje
		taskMembersUpdateFilter := bson.M{"members._id": userID}
		taskMembersUpdate := bson.M{"$pull": bson.M{"members": bson.M{"_id": userID}}}
		_, err = s.TaskCollection.UpdateMany(context.Background(), taskMembersUpdateFilter, taskMembersUpdate)
		if err != nil {
			return fmt.Errorf("failed to remove user from task members: %v", err)
		}
	}

	// Brisanje korisnika iz baze
	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
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
