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

func (s *UserService) DeleteAccount(username string) error {

	//trazi ObjectID korisnika na osnovu username
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return errors.New("user not found")
	}
	userID := user.ID

	_, err = s.UserCollection.DeleteOne(context.Background(), bson.M{"username": username})
	if err != nil {
		return errors.New("failed to delete user from users collection")
	}

	// uklanja korisnika iz members sa projekta
	projectUpdateFilter := bson.M{"members._id": userID}
	projectUpdate := bson.M{"$pull": bson.M{"members": bson.M{"_id": userID}}}
	updateResult, err := s.ProjectCollection.UpdateMany(context.Background(), projectUpdateFilter, projectUpdate)
	if err != nil {
		fmt.Println("Greška pri ažuriranju projekata:", err)
		return errors.New("failed to remove user from projects")
	}
	fmt.Printf("Korisnik uklonjen iz %d projekata.\n", updateResult.ModifiedCount)

	taskUpdateFilter := bson.M{"assignees": userID}
	taskUpdate := bson.M{"$pull": bson.M{"assignees": userID}}
	taskUpdateResult, err := s.TaskCollection.UpdateMany(context.Background(), taskUpdateFilter, taskUpdate)
	if err != nil {
		fmt.Println("Greška pri ažuriranju zadataka:", err)
		return errors.New("failed to remove user from tasks")
	}
	fmt.Printf("Korisnik uklonjen iz %d zadataka.\n", taskUpdateResult.ModifiedCount)

	// brisanje menadzera sa proj
	managerUpdateFilter := bson.M{"manager_id": userID}
	managerUpdate := bson.M{"$unset": bson.M{"manager_id": ""}}
	_, err = s.ProjectCollection.UpdateMany(context.Background(), managerUpdateFilter, managerUpdate)
	if err != nil {
		return errors.New("failed to remove manager from projects")
	}
	fmt.Println("Menadžer uspešno uklonjen iz projekata.")

	return nil

}

func (s *UserService) CanDeleteMemberAccountByUsername(username string) (bool, error) {
	fmt.Println("Proveravam da li član može biti obrisan po username...")

	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		fmt.Println("Korisnik nije pronađen:", err)
		return false, err
	}
	userID := user.ID

	taskFilter := bson.M{
		"assignees": userID,
		"status":    "in progress",
	}
	count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
	if err != nil {
		fmt.Println("Greška pri proveri zadataka:", err)
		return false, err
	}

	if count > 0 {
		return false, nil
	}

	return true, nil
}

func (s *UserService) CanDeleteManagerAccountByUsername(username string) (bool, error) {
	fmt.Println("Proveravam da li menadžer može biti obrisan po username...")

	var manager models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&manager)
	if err != nil {
		fmt.Println("Menadžer nije pronađen:", err)
		return false, err
	}
	managerID := manager.ID

	// Pronađi sve projekte koje je kreirao menadžer
	projectFilter := bson.M{"manager_id": managerID}
	cursor, err := s.ProjectCollection.Find(context.Background(), projectFilter)
	if err != nil {
		fmt.Println("Greška pri pretrazi projekata:", err)
		return false, err
	}
	defer cursor.Close(context.Background())

	var projects []models.Project
	if err := cursor.All(context.Background(), &projects); err != nil {
		fmt.Println("Greška pri učitavanju projekata:", err)
		return false, err
	}

	for _, project := range projects {
		fmt.Printf("Proveravam zadatke za projekat: %s\n", project.ID.Hex())

		taskFilter := bson.M{
			"projectId": project.ID.Hex(),
			"status":    "in progress",
		}

		count, err := s.TaskCollection.CountDocuments(context.Background(), taskFilter)
		if err != nil {
			fmt.Println("Greška pri proveri zadataka:", err)
			return false, err
		}

		if count > 0 {
			fmt.Println("Projekat ima zadatke u statusu 'Pending'.")
			return false, nil
		}
	}

	fmt.Println("Menadžer nema aktivnih zadataka u svojim projektima.")
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
