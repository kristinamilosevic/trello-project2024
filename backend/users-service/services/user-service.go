package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/rand"
)

// UserService struktura sa MongoDB kolekcijom i JWT servisom
type UserService struct {
	UserCollection *mongo.Collection
	TokenCache     map[string]string
	JWTService     *JWTService
}

// NewUserService kreira novu instancu UserService
func NewUserService(userCollection *mongo.Collection) *UserService {
	return &UserService{
		UserCollection: userCollection,
		TokenCache:     make(map[string]string), // Privremeni keš
		JWTService:     &JWTService{},           // Inicijalizacija JWT servisa
	}
}

// RegisterUser šalje verifikacioni email korisniku i čuva podatke u kešu
func (s *UserService) RegisterUser(user models.User) error {
	// Provera da li korisnik već postoji
	var existingUser models.User
	if err := s.UserCollection.FindOne(context.Background(), bson.M{"username": user.Username}).Decode(&existingUser); err == nil {
		return fmt.Errorf("User with username already exists")
	}

	// Generisanje verifikacionog koda i podešavanje vremena isteka
	verificationCode := fmt.Sprintf("%06d", rand.Intn(1000000))
	expiryTime := time.Now().Add(1 * time.Minute)

	// Postavljanje verifikacionih informacija u model korisnika
	user.VerificationCode = verificationCode
	user.VerificationExpiry = expiryTime
	user.IsActive = false

	// Čuvanje korisnika u bazi sa statusom `inactive`
	if _, err := s.UserCollection.InsertOne(context.Background(), user); err != nil {
		return fmt.Errorf("Failed to save user: %v", err)
	}

	// Slanje verifikacionog email-a sa kodom
	subject := "Your Verification Code"
	body := fmt.Sprintf("Your verification code is %s. Please enter it within 1 minute.", verificationCode)
	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %v", err)
	}

	log.Println("Verifikacioni kod poslat korisniku:", user.Email)
	return nil
}

// GetUnverifiedUserByEmail pronalazi korisnika u bazi po email adresi
func (s *UserService) GetUnverifiedUserByEmail(email string) (models.User, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		return models.User{}, fmt.Errorf("User not found")
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
		return fmt.Errorf("Failed to activate user: %v", err)
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

// LoginUser proverava kredencijale korisnika i generiše JWT token
func (s *UserService) LoginUser(email, password string) (*models.User, string, error) {
	var user models.User
	email = strings.ToLower(email)
	log.Println("Pokušavam pronaći korisnika sa emailom:", email)

	// Pronađi korisnika po email-u
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, "", errors.New("user not found")
	}

	// Proveri da li se lozinka poklapa
	if user.Password != password {
		return nil, "", errors.New("invalid password")
	}

	// Proveri da li je korisnik aktivan
	if !user.IsActive {
		return nil, "", errors.New("user not active")
	}

	// Generiši JWT token
	token, err := s.JWTService.GenerateAuthToken(user.Email, user.Role)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %v", err)
	}

	// Vraćamo korisnika i generisani token
	return &user, token, nil
}
