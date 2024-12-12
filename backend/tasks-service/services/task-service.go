package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"trello-project/microservices/tasks-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskService struct {
	tasksCollection    *mongo.Collection
	projectsCollection *mongo.Collection
}

func NewTaskService(tasksCollection, projectsCollection *mongo.Collection) *TaskService {
	return &TaskService{
		tasksCollection:    tasksCollection,
		projectsCollection: projectsCollection,
	}
}

func (s *TaskService) GetAvailableMembersForTask(projectID, taskID string) ([]models.Member, error) {
	// Pretvori projectID u ObjectID ako je potrebno
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID format: %v", err)
	}
	// Dohvatanje članova projekta
	var project struct {
		Members []models.Member `bson:"members"`
	}
	// Pretvori taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		log.Printf("Error converting taskID to ObjectID: %s, error: %v", taskID, err)
		return nil, fmt.Errorf("invalid task ID format")
	}

	log.Printf("Searching for project with projectID: %s", projectID)

	err = s.projectsCollection.FindOne(context.Background(), bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		log.Printf("Failed to fetch project members for projectID: %s, error: %v", projectID, err)
		return nil, fmt.Errorf("failed to fetch project members: %v", err)
	}

	// Dohvatanje članova zadatka
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		log.Printf("Failed to fetch task members for taskID: %s, error: %v", taskID, err)
		return nil, fmt.Errorf("failed to fetch task members: %v", err)
	}

	// Filtriraj članove koji nisu dodati na ovaj task
	// Filtriraj članove koji nisu već dodeljeni ovom zadatku
	availableMembers := []models.Member{}
	for _, member := range project.Members {
		if !containsMember(task.Members, member.ID) {
			availableMembers = append(availableMembers, member)
		}
	}

	return availableMembers, nil
}

func containsMember(members []models.Member, memberID primitive.ObjectID) bool {
	for _, m := range members {
		if m.ID == memberID {
			return true
		}
	}
	return false
}

// Dodaj članove zadatku
func (s *TaskService) AddMembersToTask(taskID string, membersToAdd []models.Member) error {
	// Konvertovanje taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID format")
	}

	// Dohvati zadatak iz baze
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return fmt.Errorf("task not found: %v", err)
	}

	// Proveri i inicijalizuj polje `members` ako je `nil`
	if task.Members == nil {
		task.Members = []models.Member{}
		_, err := s.tasksCollection.UpdateOne(
			context.Background(),
			bson.M{"_id": taskObjectID},
			bson.M{"$set": bson.M{"members": task.Members}}, // Postavi `members` kao prazan niz
		)
		if err != nil {
			return fmt.Errorf("failed to initialize members field: %v", err)
		}
	}

	// Filtriraj nove članove koji nisu već dodeljeni
	newMembers := []models.Member{}
	for _, member := range membersToAdd {
		alreadyAssigned := false
		for _, assigned := range task.Members {
			if assigned.ID == member.ID {
				alreadyAssigned = true
				break
			}
		}
		if !alreadyAssigned {
			newMembers = append(newMembers, member)
		}
	}

	if len(newMembers) > 0 {
		// Ažuriraj zadatak sa novim članovima
		update := bson.M{"$addToSet": bson.M{"members": bson.M{"$each": newMembers}}}
		_, err = s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskObjectID}, update)
		if err != nil {
			return fmt.Errorf("failed to add members to task: %v", err)
		}

		// Slanje notifikacija za nove članove
		for _, member := range newMembers {
			message := fmt.Sprintf("You have been added to the task: %s!", task.Title)
			err = s.sendNotification(member, message)
			if err != nil {
				log.Printf("Failed to send notification to member %s: %v", member.Username, err)
				// Log greške, ali ne prekidaj proces
			}
		}
	}

	return nil
}
func (s *TaskService) sendNotification(member models.Member, message string) error {
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

// GetMembersForTask vraća listu članova koji su dodati na određeni task
func (s *TaskService) GetMembersForTask(taskID primitive.ObjectID) ([]models.Member, error) {
	var task models.Task

	// Dohvati zadatak iz baze koristeći ObjectID
	err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task)
	if err != nil {
		log.Printf("Task not found: %v", err)
		return nil, fmt.Errorf("task not found")
	}

	// Vrati članove povezane sa zadatkom
	return task.Members, nil
}

func (s *TaskService) CreateTask(projectID string, title, description string, dependsOn *primitive.ObjectID, status models.TaskStatus) (*models.Task, error) {

	if dependsOn != nil {
		var dependentTask models.Task
		err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": *dependsOn}).Decode(&dependentTask)
		if err != nil {
			return nil, fmt.Errorf("dependent task not found")
		}
	}

	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID format: %v", err)
	}

	if status == "" {
		status = models.StatusPending
	}

	// Sanitizacija inputa
	sanitizedTitle := html.EscapeString(title)
	sanitizedDescription := html.EscapeString(description)

	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       sanitizedTitle,
		Description: sanitizedDescription,

		Status:    status,
		DependsOn: dependsOn,
	}

	// Unos u kolekciju zadataka
	result, err := s.tasksCollection.InsertOne(context.Background(), task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %v", err)
	}

	task.ID = result.InsertedID.(primitive.ObjectID)

	// Ažuriranje projekta sa ID-em zadatka
	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$push": bson.M{"taskIDs": task.ID}}

	_, err = s.projectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update project with task ID: %v", err)
	}

	return task, nil
}

func (s *TaskService) GetAllTasks() ([]*models.Task, error) {
	var tasks []*models.Task
	cursor, err := s.tasksCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve tasks: %v", err)
	}
	defer cursor.Close(context.Background())

	// Iteracija kroz sve zadatke
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

func (s *TaskService) RemoveMemberFromTask(taskID string, memberID primitive.ObjectID) error {
	// Konvertovanje taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID format")
	}

	// Dohvatanje zadatka iz baze
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return fmt.Errorf("task not found: %v", err)
	}

	// Provera da li je član deo zadatka
	memberFound := false
	var removedMember models.Member
	for i, member := range task.Members {
		if member.ID == memberID {
			removedMember = member
			task.Members = append(task.Members[:i], task.Members[i+1:]...)
			memberFound = true
			break
		}
	}

	if !memberFound {
		return fmt.Errorf("member not found in the task")
	}

	// Ažuriranje zadatka u bazi
	_, err = s.tasksCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": taskObjectID},
		bson.M{"$set": bson.M{"members": task.Members}},
	)
	if err != nil {
		return fmt.Errorf("failed to update task: %v", err)
	}

	// Slanje notifikacije uklonjenom članu
	message := fmt.Sprintf("You have been removed from the task: %s", task.Title)
	err = s.sendNotification(removedMember, message)
	if err != nil {
		log.Printf("Failed to send notification to member %s: %v", removedMember.Username, err)
	}

	return nil
}

func (s *TaskService) GetTaskByID(taskID primitive.ObjectID) (*models.Task, error) {
	var task models.Task
	err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *TaskService) GetTasksByProject(projectID string) ([]*models.Task, error) {
	var tasks []*models.Task

	filter := bson.M{"projectId": projectID}
	cursor, err := s.tasksCollection.Find(context.Background(), filter)
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

func (s *TaskService) ChangeTaskStatus(taskID primitive.ObjectID, status models.TaskStatus, username string) (*models.Task, error) {
	// Pronađi zadatak u bazi
	var task models.Task
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("task not found: %v", err)
	}

	fmt.Printf("Task '%s' current status: %s\n", task.Title, task.Status)
	fmt.Printf("Attempting to change status to: %s\n", status)

	// Proveri da li je korisnik zadužen za zadatak
	var isAuthorized bool
	for _, member := range task.Members {
		if member.Username == username {
			isAuthorized = true
			break
		}
	}

	if !isAuthorized {
		return nil, fmt.Errorf("user '%s' is not authorized to change the status of this task because they are not assigned to it", username)
	}

	// Ako postoji zavisni zadatak, proveri njegov status
	if task.DependsOn != nil {
		var dependentTask models.Task
		if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": task.DependsOn}).Decode(&dependentTask); err != nil {
			return nil, fmt.Errorf("dependent task not found: %v", err)
		}

		if dependentTask.Status != models.StatusInProgress && dependentTask.Status != models.StatusCompleted && status != models.StatusPending {
			return nil, fmt.Errorf("cannot change status because dependent task '%s' is not in progress or completed", dependentTask.Title)
		}
	}

	// Ažuriraj status trenutnog zadatka
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskID}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %v", err)
	}

	// Osveži podatke zadatka nakon promene statusa
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to retrieve updated task: %v", err)
	}

	fmt.Printf("Status of task '%s' successfully updated to: %s\n", task.Title, status)

	// Pošalji notifikacije svim članovima zadatka
	message := fmt.Sprintf("The status of the task '%s' has been changed to: %s", task.Title, status)
	for _, member := range task.Members {
		err := s.sendNotification(member, message)
		if err != nil {
			log.Printf("Failed to send notification to member %s: %v", member.Username, err)
		}
	}

	return &task, nil
}
func (s *TaskService) DeleteTasksByProject(projectID string) error {
	// Filter za pronalaženje zadataka sa projectId
	filter := bson.M{"projectId": projectID}

	// Brisanje svih zadataka vezanih za projekat
	result, err := s.tasksCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		log.Printf("Failed to delete tasks for project ID %s: %v", projectID, err)
		return fmt.Errorf("failed to delete tasks: %v", err)
	}

	log.Printf("Successfully deleted %d tasks for project ID %s", result.DeletedCount, projectID)
	return nil
}
