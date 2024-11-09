package services

import (
	"context"
	"fmt"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserService struct {
	UserCollection *mongo.Collection
	JWTService     *JWTService
	TokenCache     map[string]string
}

func NewUserService(userCollection *mongo.Collection) *UserService {
	return &UserService{
		UserCollection: userCollection,
		JWTService:     &JWTService{},
		TokenCache:     make(map[string]string), // Privremeni keš
	}
}

// RegisterUser šalje verifikacioni email korisniku bez čuvanja u bazi
func (s *UserService) RegisterUser(user models.User) error {
	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"email": user.Email}).Decode(&existingUser)
	if err != mongo.ErrNoDocuments {
		return fmt.Errorf("User with email already exists")
	}

	// Generisanje JWT tokena
	token, err := s.JWTService.GenerateEmailVerificationToken(user.Email)
	if err != nil {
		return fmt.Errorf("Failed to generate token: %v", err)
	}

	// Sačuvajte korisničke podatke i token u kešu
	s.TokenCache[user.Email] = fmt.Sprintf("%s|%s|%s|%s|%s|%s", token, user.Name, user.LastName, user.Username, user.Password, user.Role)

	// Slanje emaila sa linkom za potvrdu
	verificationLink := "http://localhost:4200/projects-list?verify=true"
	subject := "Confirm your email"
	body := fmt.Sprintf("Click the link to confirm your registration: %s", verificationLink)

	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("Failed to send email: %v", err)
	}

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
func (s *UserService) CreateUser(user models.User) error {
	_, err := s.UserCollection.InsertOne(context.Background(), user)
	return err
}
