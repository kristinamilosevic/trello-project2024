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

func (s *TaskService) CreateTask(projectID string, title, description string, status models.TaskStatus) (*models.Task, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID format: %v", err)
	}

	if status == "" {
		status = models.StatusPending
	}

	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      status,
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

func (s *TaskService) GetTaskByID(taskID primitive.ObjectID) (*models.Task, error) {
	var task models.Task
	err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ChangeTaskStatus - menja status taska
func (s *TaskService) ChangeTaskStatus(taskID primitive.ObjectID, status models.TaskStatus) (*models.Task, error) {
	// Pronađi task po ID-u
	var task models.Task
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("task not found: %v", err)
	}

	// Proveri zavisnost
	if task.DependsOn != nil {
		var dependentTask models.Task
		if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": *task.DependsOn}).Decode(&dependentTask); err != nil {
			return nil, fmt.Errorf("dependent task not found: %v", err)
		}

		if dependentTask.Status != models.StatusCompleted && status == models.StatusInProgress {
			return nil, fmt.Errorf("cannot start task due to unfinished dependency")
		}
	}

	// Ažuriraj status taska
	update := bson.M{"$set": bson.M{"status": status}}
	result, err := s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskID}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %v", err)
	}

	// Proveri da li je dokument zaista ažuriran
	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("task not found for update")
	}
	if result.ModifiedCount == 0 {
		return nil, fmt.Errorf("task status not updated")
	}

	// Vratimo ažurirani task
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to retrieve updated task: %v", err)
	}

	return &task, nil
}
