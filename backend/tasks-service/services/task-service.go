package services

import (
	"context"
	"fmt"
	"trello-project/microservices/tasks-service/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskService struct {
	collection *mongo.Collection
}

// Kreirajte novi TaskService
func NewTaskService(client *mongo.Client) *TaskService {
	return &TaskService{collection: client.Database("tasks_db").Collection("tasks")}
}

// Funkcija za kreiranje novog zadatka
func (s *TaskService) CreateTask(projectID, title, description string) (*models.Task, error) {
	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      "Pending", // Inicijalni status
	}

	result, err := s.collection.InsertOne(context.Background(), task)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %v", err)
	}

	task.ID = result.InsertedID.(primitive.ObjectID)
	return task, nil
}
