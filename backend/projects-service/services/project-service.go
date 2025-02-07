package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"time"
	"trello-project/microservices/projects-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	ProjectsCollection *mongo.Collection
	TasksCollection    *mongo.Collection
	UsersCollection    *mongo.Collection
}

// NewProjectService initializes a new ProjectService with the necessary MongoDB collections.
func NewProjectService(projectsCollection, usersCollection, tasksCollection *mongo.Collection) *ProjectService {
	return &ProjectService{
		ProjectsCollection: projectsCollection,
		UsersCollection:    usersCollection,
		TasksCollection:    tasksCollection,
	}
}

// CreateProject creates a new project with the specified parameters.
func (s *ProjectService) CreateProject(name string, description string, expectedEndDate time.Time, minMembers, maxMembers int, managerID primitive.ObjectID) (*models.Project, error) {
	var existingProject models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"name": name}).Decode(&existingProject)
	if err == nil {
		return nil, errors.New("project with the same name already exists")
	}

	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database error: %v", err)
	}
	// Validate input parameters
	if minMembers < 1 || maxMembers < minMembers {
		return nil, fmt.Errorf("invalid member constraints: minMembers=%d, maxMembers=%d", minMembers, maxMembers)
	}
	if expectedEndDate.Before(time.Now()) {
		return nil, fmt.Errorf("expected end date must be in the future")
	}
	// Sanitizacija inputa
	sanitizedName := html.EscapeString(name)
	sanitizedDescription := html.EscapeString(description)

	// Create the project
	project := &models.Project{
		ID:              primitive.NewObjectID(),
		Name:            sanitizedName,
		Description:     sanitizedDescription,
		ExpectedEndDate: expectedEndDate,
		MinMembers:      minMembers,
		MaxMembers:      maxMembers,
		ManagerID:       managerID,
		Members:         []models.Member{},
		Tasks:           []primitive.ObjectID{},
	}

	// Insert the project into the collection
	result, err := s.ProjectsCollection.InsertOne(context.Background(), project)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %v", err)
	}

	// Set the project ID from the inserted result
	project.ID = result.InsertedID.(primitive.ObjectID)
	return project, nil
}

// AddMembersToProject adds multiple members to a project.
func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, memberIDs []primitive.ObjectID) error {
	var project models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": projectID}).Decode(&project)
	if err != nil {
		fmt.Printf("Error finding project: %v\n", err)
		return fmt.Errorf("project not found: %v", err)
	}

	// Provera maksimalnog broja članova
	if len(project.Members)+len(memberIDs) > project.MaxMembers {
		fmt.Println("Maximum number of members reached for the project")
		return fmt.Errorf("maximum number of members reached for the project")
	}

	// Filtriranje članova koji su već na projektu
	existingMemberIDs := make(map[primitive.ObjectID]bool)
	for _, member := range project.Members {
		existingMemberIDs[member.ID] = true
	}

	var newMemberIDs []primitive.ObjectID
	for _, memberID := range memberIDs {
		if !existingMemberIDs[memberID] {
			newMemberIDs = append(newMemberIDs, memberID)
		} else {
			fmt.Printf("Member %s is already in the project, skipping.\n", memberID.Hex())
		}
	}

	if len(newMemberIDs) == 0 {
		fmt.Println("No new members to add.")
		return fmt.Errorf("all provided members are already part of the project")
	}

	// Dohvatanje korisničkih podataka i priprema za ažuriranje
	var members []models.Member
	for _, memberID := range newMemberIDs {
		var user models.Member
		err := s.UsersCollection.FindOne(context.Background(), bson.M{"_id": memberID}).Decode(&user)
		if err != nil {
			fmt.Printf("Error finding member: %v\n", err)
			return err // Greška ako član nije pronađen
		}
		members = append(members, user)
	}

	// Ažuriranje baze sa novim članovima
	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err = s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		fmt.Printf("Error updating project members: %v\n", err)
		return err
	}

	// Slanje notifikacija za svakog novog člana
	for _, member := range members {
		message := fmt.Sprintf("You have been added to the project: %s!", project.Name)
		err = s.sendNotification(member, message)
		if err != nil {
			fmt.Printf("Failed to send notification to member %s: %v\n", member.Username, err)
			// Loguj grešku i nastavi sa ostalim članovima
		}
	}

	fmt.Println("Members successfully added and notifications sent.")
	return nil
}

func (s *ProjectService) sendNotification(member models.Member, message string) error {
	notification := map[string]string{
		"userId":   member.ID.Hex(),
		"username": member.Username,
		"message":  message, // Dinamična poruka
	}

	notificationData, err := json.Marshal(notification)
	if err != nil {
		fmt.Printf("Error marshaling notification data: %v\n", err)
		return nil
	}

	// Učitaj URL iz .env fajla
	notificationURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")
	if notificationURL == "" {
		fmt.Println("Notification service URL is not set in .env")
		return fmt.Errorf("notification service URL is not configured")
	}

	req, err := http.NewRequest("POST", notificationURL, bytes.NewBuffer(notificationData))
	if err != nil {
		fmt.Printf("Error creating new request: %v\n", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Role", "manager")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error sending HTTP request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("Failed to create notification, status code: %d\n", resp.StatusCode)
		return nil
	}

	fmt.Printf("Notification successfully sent for member: %s\n", member.Username)
	return nil
}

// GetProjectMembers retrieves members of a specific project.
func (s *ProjectService) GetProjectMembers(ctx context.Context, projectID string) ([]bson.M, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return nil, err
	}

	var project struct {
		Members []bson.M `bson:"members"`
	}

	err = s.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		fmt.Println("Error fetching project members from database:", err)
		return nil, err
	}

	return project.Members, nil
}

// GetAllUsers retrieves all users from the users collection.
func (s *ProjectService) GetAllUsers() ([]models.Member, error) {
	var users []models.Member
	cursor, err := s.UsersCollection.Find(context.Background(), bson.M{})
	if err != nil {
		fmt.Println("Error finding users:", err) // Log greške pri dohvaćanju korisnika
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &users); err != nil {
		fmt.Println("Error decoding users:", err) // Log greške pri dekodiranju korisnika
		return nil, err
	}

	fmt.Println("Fetched users:", users) // Log za proveru vraćenih korisnika
	return users, nil
}

// RemoveMemberFromProject removes a member from a project if they are not assigned to an in-progress task.
func (s *ProjectService) RemoveMemberFromProject(ctx context.Context, projectID, memberID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return errors.New("invalid project ID format")
	}

	memberObjectID, err := primitive.ObjectIDFromHex(memberID)
	if err != nil {
		return errors.New("invalid member ID format")
	}

	// Proverite da li član ima aktivne zadatke
	taskFilter := bson.M{
		"projectId": projectObjectID.Hex(), // ID projekta
		"status":    "in progress",
		"assignees": memberObjectID, // ID člana
	}

	cursor, err := s.TasksCollection.Find(ctx, taskFilter)
	if err != nil {
		return errors.New("failed to check task assignments")
	}
	defer cursor.Close(ctx)

	// Proverite ima li aktivnih zadataka
	if cursor.TryNext(ctx) { // Ako postoje rezultati
		return errors.New("cannot remove member assigned to an in-progress task")
	}

	// Dohvati projekat za ime projekta
	var project models.Project
	err = s.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		return errors.New("project not found")
	}

	// Dohvati podatke o članu za notifikaciju
	var member models.Member
	err = s.UsersCollection.FindOne(ctx, bson.M{"_id": memberObjectID}).Decode(&member)
	if err != nil {
		return errors.New("member not found")
	}

	// Uklonite člana iz projekta
	projectFilter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"_id": memberObjectID}}}

	result, err := s.ProjectsCollection.UpdateOne(ctx, projectFilter, update)
	if err != nil {
		return errors.New("failed to remove member from project")
	}

	if result.ModifiedCount == 0 {
		return errors.New("member not found in project or already removed")
	}

	// Slanje notifikacije nakon uspešnog uklanjanja
	message := fmt.Sprintf("You have been removed from the project: %s!", project.Name)
	err = s.sendNotification(member, message)
	if err != nil {
		log.Printf("Failed to send notification to member %s: %v\n", member.Username, err)
		// Log greške, ali ne prekidaj proces
	}

	return nil
}

// GetAllProjects - preuzima sve projekte iz kolekcije
func (s *ProjectService) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	cursor, err := s.ProjectsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("unsuccessful procurement of projects: %v", err)
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &projects); err != nil {
		return nil, fmt.Errorf("unsuccessful decoding of projects: %v", err)
	}

	return projects, nil
}

func (s *ProjectService) GetProjectByID(projectID string) (*models.Project, error) {
	objectId, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		fmt.Println("Invalid project ID format:", projectID)
		return nil, fmt.Errorf("invalid project ID format")
	}

	var project models.Project
	err = s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("error fetching project: %v", err)
	}
	return &project, nil
}

func (s *ProjectService) GetTasksForProject(projectID primitive.ObjectID) ([]*models.Task, error) {
	var project models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": projectID}).Decode(&project)
	if err != nil {
		return nil, fmt.Errorf("project not found: %v", err)
	}

	var tasks []*models.Task
	filter := bson.M{"_id": bson.M{"$in": project.Tasks}}
	cursor, err := s.TasksCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tasks: %v", err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var task models.Task
		if err := cursor.Decode(&task); err != nil {
			return nil, fmt.Errorf("failed to decode task: %v", err)
		}
		tasks = append(tasks, &task)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	return tasks, nil
}

func (s *ProjectService) GetProjectsByUsername(username string) ([]models.Project, error) {
	var projects []models.Project
	filter := bson.M{"members.username": username}

	log.Printf("Executing MongoDB query with filter: %v", filter)

	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		log.Printf("Error fetching projects from MongoDB: %v", err)
		return nil, fmt.Errorf("error fetching projects: %v", err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			log.Printf("Error decoding project document: %v", err)
			return nil, fmt.Errorf("error decoding project: %v", err)
		}
		projects = append(projects, project)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	log.Printf("Found %d projects for username %s", len(projects), username)
	return projects, nil
}
func (s *ProjectService) AddTaskToProject(projectID primitive.ObjectID, taskID string) error {
	filter := bson.M{"_id": projectID}
	update := bson.M{"$addToSet": bson.M{"tasks": taskID}}

	_, err := s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return fmt.Errorf("failed to update project with task ID: %v", err)
	}

	log.Printf("Task ID %s successfully added to project %s", taskID, projectID.Hex())
	return nil
}
