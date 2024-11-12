package services

import (
	"context"
	"errors"
	"fmt"
	"log"

	"time"

	"trello-project/microservices/users-service/models"
	"trello-project/microservices/users-service/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/exp/rand"
)

// UserService struktura
type UserService struct {
	UserCollection    *mongo.Collection
	TokenCache        map[string]string
	JWTService        *JWTService
	ProjectCollection *mongo.Collection
	TaskCollection    *mongo.Collection
}

func NewUserService(userCollection, projectCollection, taskCollection *mongo.Collection, jwtService *JWTService) *UserService {
	return &UserService{

		UserCollection:    userCollection,
		TokenCache:        make(map[string]string),
		JWTService:        &JWTService{},
		ProjectCollection: projectCollection,
		TaskCollection:    taskCollection,
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

func (s UserService) LoginUser(username, password string) (models.User, string, error) {
	var user models.User
	err := s.UserCollection.FindOne(context.Background(), bson.M{"username": username}).Decode(&user)
	if err != nil {
		return models.User{}, "", errors.New("user not found")
	}
	if user.Password != password {
		return models.User{}, "", errors.New("invalid password")
	}
	if !user.IsActive {
		return models.User{}, "", errors.New("user not active")
	}
	token, err := s.JWTService.GenerateAuthToken(user.Username, user.Role)
	if err != nil {
		return models.User{}, "", fmt.Errorf("failed to generate token: %v", err)
	}
	return user, token, nil
}

// DeleteExpiredUnverifiedUsers briše korisnike kojima je istekao rok za verifikaciju i koji nisu aktivni
func (s *UserService) DeleteExpiredUnverifiedUsers() {
	filter := bson.M{
		"isActive": false,
		"verificationExpiry": bson.M{
			"$lt": time.Now(),
		},
	}

	// Brišemo sve korisnike koji odgovaraju uslovima
	result, err := s.UserCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		log.Printf("Greška prilikom brisanja korisnika sa isteklim verifikacionim rokom: %v", err)
	} else {
		log.Printf("Obrisano %d korisnika sa isteklim verifikacionim rokom.", result.DeletedCount)
	}
}
