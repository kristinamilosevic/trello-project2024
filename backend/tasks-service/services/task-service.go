package services

import (
	"context"
	"fmt"
	"strings"
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

	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       title,
		Description: description,
		Status:      status,
		DependsOn:   dependsOn,
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

func (s *TaskService) ChangeTaskStatus(taskID primitive.ObjectID, status models.TaskStatus) (*models.Task, error) {
	var task models.Task
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("task not found: %v", err)
	}

	fmt.Printf("Task '%s' current status: %s\n", task.Title, task.Status)
	fmt.Printf("Attempting to change status to: %s\n", status)

	//d
	if task.DependsOn != nil {
		fmt.Printf("Checking dependent task with ID: %v\n", task.DependsOn)

		// Pronadji zavisni task u bazi
		var dependentTask models.Task
		if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": task.DependsOn}).Decode(&dependentTask); err != nil {
			return nil, fmt.Errorf("dependent task not found: %v", err)
		}

		fmt.Printf("Dependent task '%s' status: '%s'\n", dependentTask.Title, dependentTask.Status)
		fmt.Printf("Expected status for comparison: '%s'\n", models.StatusCompleted)

		// Ako zavisni task nije zavrsen promena statusa nije dozvoljena
		if strings.TrimSpace(string(dependentTask.Status)) != string(models.StatusCompleted) && status != models.StatusPending {
			return nil, fmt.Errorf("cannot change status because dependent task '%s' is not completed", dependentTask.Title)
		}
	}

	// update status trenutnog taska
	update := bson.M{"$set": bson.M{"status": status}}
	_, err := s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskID}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %v", err)
	}

	fmt.Printf("Status of task '%s' successfully updated to: %s\n", task.Title, status)

	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to retrieve updated task: %v", err)
	}

	return &task, nil
}
