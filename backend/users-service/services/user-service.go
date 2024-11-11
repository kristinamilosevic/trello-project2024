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

// Funkcija za brisanje naloga (menadžera ili člana)
func (s *UserService) DeleteAccount(userID primitive.ObjectID, role string) error {

	var existingUser models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"_id": userID}).Decode(&existingUser)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return errors.New("user does not exist in the database")
		}
		return err
	}

	fmt.Println("Korisnik pronađen:", existingUser.Name, "sa ulogom:", role)

	if role == "manager" {
		canDelete, err := s.CanDeleteManagerAccount(userID)
		if err != nil {
			fmt.Println("Greška pri proveri menadžera:", err)
			return err
		}
		if !canDelete {
			return errors.New("cannot delete manager account with active projects")
		}
	} else if role == "member" {
		canDelete, err := s.CanDeleteMemberAccount(userID)
		if err != nil {
			fmt.Println("Greška pri proveri člana:", err)
			return err
		}
		if !canDelete {
			return errors.New("cannot delete member account assigned to active projects")
		}
	}

	fmt.Println("Brisanje korisnika iz baze:", userID.Hex())
	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"_id": userID})
	if err != nil {
		fmt.Println("Greška pri brisanju korisnika:", err)
		return errors.New("failed to delete user")
	}

	fmt.Println("Korisnik uspešno obrisan:", userID.Hex())
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

	// Pronađi sve projekte gde je član
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
