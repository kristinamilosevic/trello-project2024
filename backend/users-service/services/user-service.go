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

type UserService struct {
	UserCollection    *mongo.Collection
	JWTService        *JWTService
	ProjectCollection *mongo.Collection
	TaskCollection    *mongo.Collection
}

func NewUserService(userCollection, projectCollection, taskCollection *mongo.Collection) *UserService {
	return &UserService{
		UserCollection:    userCollection,
		ProjectCollection: projectCollection,
		TaskCollection:    taskCollection,
		JWTService:        &JWTService{},
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

// Funkcija za brisanje menadžerskog naloga
func (s *UserService) DeleteManagerAccount(managerID primitive.ObjectID) error {
	// 1. Proveri da li korisnik postoji u bazi
	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"_id": managerID}).Decode(&existingUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user does not exist in the database")
		}
		return err
	}

	// 2. Proveri da li menadžer može obrisati nalog (aktivni projekti ili zadaci)
	canDelete, err := s.CanDeleteManagerAccount(managerID)
	if err != nil {
		return err
	}
	if !canDelete {
		return errors.New("cannot delete account with active projects")
	}

	// 3. Fizičko brisanje korisnika iz baze
	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"_id": managerID})
	if err != nil {
		return errors.New("failed to delete user")
	}

	return nil
}

// Proverava da li menadžer može obrisati nalog
func (s *UserService) CanDeleteManagerAccount(managerID primitive.ObjectID) (bool, error) {

	//  Pronađi sve projekte koje vodi menadzer
	projectFilter := bson.M{"manager_id": managerID}
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
		projectIDStr := project.ID.Hex()

		taskCursor, err := s.TaskCollection.Find(context.Background(), bson.M{"projectId": projectIDStr})
		if err != nil {
			return false, err
		}
		defer taskCursor.Close(context.Background())

		var tasks []models.Task
		if err := taskCursor.All(context.Background(), &tasks); err != nil {
			return false, err
		}

		for _, task := range tasks {
			if task.Status == "in progress" {
				fmt.Println("Zadatak je u statusu 'in progress', menadžer ne može biti obrisan.")
				return false, nil
			}
		}
	}
	// Proveri zadatke dodeljene menadžeru koristeci managerID
	managerTaskFilter := bson.M{
		"assignees": managerID,
		"status":    "in progress",
	}
	cursor, err = s.TaskCollection.Find(context.Background(), managerTaskFilter)
	if err != nil {
		return false, err
	}
	defer cursor.Close(context.Background())

	var assignedTasks []models.Task
	if err := cursor.All(context.Background(), &assignedTasks); err != nil {
		return false, err
	}

	for _, task := range assignedTasks {
		fmt.Printf("Assigned Task ID: %s, Status: %s\n", task.ID.Hex(), task.Status)
		if task.Status == "in progress" {
			return false, nil
		}
	}

	return true, nil
}
