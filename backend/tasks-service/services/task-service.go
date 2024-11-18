package services

import (
	"context"
	"fmt"
	"trello-project/microservices/tasks-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskService struct {
	tasksCollection    *mongo.Collection
	projectsCollection *mongo.Collection
}

func NewTaskService(client *mongo.Client) *TaskService {
	return &TaskService{
		tasksCollection:    client.Database("tasks_db").Collection("tasks"),
		projectsCollection: client.Database("projects_db").Collection("project"),
	}
}

// Dohvati članove projekta koji nisu dodati zadatku
func (s *TaskService) GetAvailableMembersForTask(projectID, taskID string) ([]models.Member, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID format")
	}
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID format")
	}

	// Dohvatanje članova projekta
	var project struct {
		Members []models.Member `bson:"members"`
	}
	err = s.projectsCollection.FindOne(context.Background(), bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project members: %v", err)
	}

	// Dohvatanje članova zadatka
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch task members: %v", err)
	}

	// Filtriranje članova koji još nisu dodati zadatku
	var availableMembers []models.Member
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
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID format")
	}

	// Dohvatamo task iz baze
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return fmt.Errorf("task not found: %v", err)
	}

	// Filtriramo članove koji su već dodeljeni zadatku
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

	if len(newMembers) == 0 {
		return fmt.Errorf("no new members to add")
	}

	update := bson.M{"$addToSet": bson.M{"members": bson.M{"$each": newMembers}}}
	_, err = s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskObjectID}, update)
	if err != nil {
		return fmt.Errorf("failed to add members to task: %v", err)
	}

	return nil
}

// GetMembersForTask vraća listu članova koji su dodati na određeni task
func (s *TaskService) GetMembersForTask(taskID string) ([]models.Member, error) {
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task ID format")
	}

	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	// Vraćamo članove dodeljene tom tasku
	return task.Assignees, nil
}

func (s *TaskService) CreateTask(projectID string, title, description string) (*models.Task, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID format: %v", err)
	}

	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      "Pending",
	}

	result, err := s.tasksCollection.InsertOne(context.Background(), task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %v", err)
	}

	task.ID = result.InsertedID.(primitive.ObjectID)

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
