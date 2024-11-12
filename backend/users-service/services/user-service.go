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
func (s *UserService) DeleteAccount(username, role string) error {
	fmt.Println("Brisanje naloga za:", username)

	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&existingUser)
	if err != nil {
		return errors.New("user not found")
	}

	if role == "manager" {
		canDelete, err := s.CanDeleteManagerAccount(existingUser.ID)
		if err != nil || !canDelete {
			return errors.New("cannot delete manager account with active projects")
		}
	} else if role == "member" {
		canDelete, err := s.CanDeleteMemberAccount(existingUser.ID)
		if err != nil || !canDelete {
			return errors.New("cannot delete member account assigned to active projects")
		}
	}

	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		return errors.New("failed to delete user")
	}

	fmt.Println("Korisnik uspešno obrisan:", username)
	return nil
}

func (s *UserService) CanDeleteManagerAccount(managerID primitive.ObjectID) (bool, error) {
	fmt.Println("Proveravam da li menadžer može biti obrisan...")

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
		fmt.Println("Proveravam projekt:", projectIDStr)

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

	managerTaskFilter := bson.M{"assignees": managerID, "status": "in progress"}
	count, err := s.TaskCollection.CountDocuments(context.Background(), managerTaskFilter)
	if err != nil || count > 0 {
		fmt.Println("Menadžer ima dodeljene zadatke u statusu 'in progress'")
		return false, nil
	}

	fmt.Println("Menadžer može biti obrisan.")
	return true, nil
}

func (s *UserService) CanDeleteMemberAccount(memberID primitive.ObjectID) (bool, error) {
	fmt.Println("Proveravam da li član može biti obrisan...")

	// Pronađi sve projekte gde se pojavljuje član sa datim ID-jem
	projectFilter := bson.M{"members._id": memberID}
	cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
	if err != nil {
		return false, err
	}
	defer cursor.Close(context.Background())

	var projects []models.Project
	if err := cursor.All(context.Background(), &projects); err != nil {
		return false, err
	}

	// Proveri zadatke za projekte u kojima je član dodeljen
	for _, project := range projects {
		fmt.Println("Proveravam projekt:", project.ID.Hex())

		taskFilter := bson.M{
			"projectId": project.ID.Hex(),
			"assignees": memberID,
			"status":    "in progress",
		}
		count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
		if err != nil || count > 0 {
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
