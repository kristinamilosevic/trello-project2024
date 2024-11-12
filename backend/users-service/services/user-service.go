package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// UserService struktura
type UserService struct {
	UserCollection    *mongo.Collection
	ProjectCollection *mongo.Collection
	TaskCollection    *mongo.Collection
	JWTService        *JWTService
}

func NewUserService(userCollection, projectCollection, taskCollection *mongo.Collection, jwtService *JWTService) *UserService {
	return &UserService{
		UserCollection:    userCollection,
		ProjectCollection: projectCollection,
		TaskCollection:    taskCollection,
		JWTService:        jwtService,
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

// Funkcija za brisanje naloga koristeći JWT token
func (s *UserService) DeleteAccount(username string) error {
	fmt.Println("Brisanje naloga za:", username)

	// Pronađi korisnika pre brisanja
	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&existingUser)
	if err != nil {
		return errors.New("user not found")
	}

	// Briši korisnika iz baze podataka
	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		return errors.New("failed to delete user")
	}

	fmt.Println("Korisnik uspešno obrisan:", username)
	return nil
}

func (s *UserService) CanDeleteManagerAccountByUsername(username string) (bool, error) {
	fmt.Println("Proveravam da li menadžer može biti obrisan po username...")

	projectFilter := bson.M{"manager_username": username}
	cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
	if err != nil {
		return false, err
	}
	defer cursor.Close(context.Background())

	var projects []models.Project
	if err := cursor.All(context.Background(), &projects); err != nil {
		return false, err
	}

	for _, project := range projects {
		taskFilter := bson.M{
			"projectId": project.ID.Hex(),
			"status":    "in progress",
		}
		count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
		if err != nil {
			return false, err
		}

		// Ako postoji zadatak u toku, zabrani brisanje
		if count > 0 {
			fmt.Println("Menadžer ima aktivne zadatke u toku.")
			return false, nil
		}
	}

	return true, nil
}

func (s *UserService) CanDeleteMemberAccountByUsername(username string) (bool, error) {
	fmt.Println("Proveravam da li član može biti obrisan po username...")

	// Pronađi sve projekte gde se pojavljuje član sa zadatim username-om
	projectFilter := bson.M{"members.username": username}
	cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
	if err != nil {
		return false, err
	}
	defer cursor.Close(context.Background())

	var projects []models.Project
	if err := cursor.All(context.Background(), &projects); err != nil {
		return false, err
	}

	// Proveri sve projekte za zadatke u toku
	for _, project := range projects {
		taskFilter := bson.M{
			"projectId":          project.ID.Hex(),
			"assignees.username": username,
			"status":             "in progress",
		}
		count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
		if err != nil {
			return false, err
		}

		// Ako postoji zadatak u toku, zabrani brisanje
		if count > 0 {
			fmt.Println("Član je dodeljen zadatku u toku.")
			return false, nil
		}
	}

	return true, nil
}

// ResetPasswordByUsername resetuje lozinku korisnika i šalje novu lozinku na email
func (s *UserService) ResetPasswordByUsername(username string) error {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if !user.IsActive {
		return errors.New("user is not active")
	}

	// Generiši novu lozinku
	newPassword := utils.GenerateRandomPassword()

	// Ažuriraj lozinku u bazi
	_, err = s.UserCollection.UpdateOne(context.Background(), bson.M{"username": username}, bson.M{"$set": bson.M{"password": newPassword}})
	if err != nil {
		return fmt.Errorf("failed to update password: %v", err)
	}

	// Pošalji novu lozinku korisniku na email
	subject := "Your new password"
	body := fmt.Sprintf("Your new password is: %s", newPassword)
	if err := utils.SendEmail(user.Email, subject, body); err != nil {
		return fmt.Errorf("failed to send email: %v", err)
	}

	log.Println("New password sent to:", user.Email)
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
