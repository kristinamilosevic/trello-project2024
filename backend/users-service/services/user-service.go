package services

import (
	"context"
	"fmt"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserService struct {
	UserCollection *mongo.Collection
	JWTService     *JWTService
}

func NewUserService(userCollection *mongo.Collection) *UserService {
	return &UserService{
		UserCollection: userCollection,
		JWTService:     &JWTService{},
	}
}

func (s *UserService) RegisterUser(user models.User) error {
	// Provera da li korisnik već postoji na osnovu email-a
	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": user.Email}).Decode(&existingUser)
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("User with email already exists")
	}

	// Inicijalno postavljanje korisnika kao neaktivan
	user.ID = primitive.NewObjectID()
	user.IsActive = false

	// Unos korisnika u bazu
	_, err = s.UserCollection.InsertOne(context.Background(), user)
	if err != nil {
		return err
	}

	// Generisanje JWT tokena
	token, err := s.JWTService.GenerateEmailVerificationToken(user.Email)
	if err != nil {
		return fmt.Errorf("Failed to generate token: %v", err)
	}

	// Slanje emaila za potvrdu registracije
	verificationLink := fmt.Sprintf("http://localhost:4200/project-list?token=%s", token)
	subject := "Confirm your email"
	body := fmt.Sprintf("Click the link to confirm your registration: %s", verificationLink)

	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %v", err)
	}

	return nil
}
