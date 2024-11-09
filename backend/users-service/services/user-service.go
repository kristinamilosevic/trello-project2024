package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UserService struktura sa MongoDB kolekcijom i JWT servisom
type UserService struct {
	UserCollection *mongo.Collection
	JWTService     *JWTService
}

// NewUserService kreira novu instancu UserService
func NewUserService(userCollection *mongo.Collection) *UserService {
	return &UserService{
		UserCollection: userCollection,
		JWTService:     &JWTService{},
	}
}

// RegisterUser registruje novog korisnika
func (s *UserService) RegisterUser(user models.User) error {
	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": user.Email}).Decode(&existingUser)
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("User with email already exists")
	}

	user.ID = primitive.NewObjectID()
	user.IsActive = false

	_, err = s.UserCollection.InsertOne(context.Background(), user)
	if err != nil {
		return err
	}

	token, err := s.JWTService.GenerateEmailVerificationToken(user.Email)
	if err != nil {
		return fmt.Errorf("Failed to generate token: %v", err)
	}

	verificationLink := fmt.Sprintf("http://localhost:4200/project-list?token=%s", token)
	subject := "Confirm your email"
	body := fmt.Sprintf("Click the link to confirm your registration: %s", verificationLink)

	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %v", err)
	}

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
