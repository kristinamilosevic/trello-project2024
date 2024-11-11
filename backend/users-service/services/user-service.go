package services

import (
	"context"
	"errors"
	"fmt"

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

func (s *UserService) SendPasswordResetLink(email string) error {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
	if err != nil {
		return errors.New("user not found")

	}
	if !user.IsActive {
		return errors.New("user is not active")
	}

	token, err := s.JWTService.GenerateEmailVerificationToken(email)
	if err != nil {
		return fmt.Errorf("failed to generate token: %v", err)
	}

	resetLink := fmt.Sprintf("http://localhost:4200/reset-password?token=%s", token)
	subject := "Reset your password"
	body := fmt.Sprintf("Click the link to reset your password: %s", resetLink)
	if err := utils.SendEmail(email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email")
	}

	return nil
}

func (s *UserService) LoginUser(username, password string) (*models.User, string, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return nil, "", errors.New("user not found")
	}
	if user.Password != password {
		return nil, "", errors.New("invalid password")
	}
	if !user.IsActive {
		return nil, "", errors.New("user not active")
	}
	token, err := s.JWTService.GenerateAuthToken(user.Username, user.Role)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate token: %v", err)
	}
	return &user, token, nil
}
