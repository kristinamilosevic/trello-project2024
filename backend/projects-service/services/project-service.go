package services

import (
	"context"
	"errors"
	"fmt"
	"time"
	"trello-project/microservices/projects-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	ProjectsCollection *mongo.Collection
	TasksCollection    *mongo.Collection
}

// NewProjectService initializes a new ProjectService with the necessary MongoDB collections.
func NewProjectService(client *mongo.Client) *ProjectService {
	return &ProjectService{
		ProjectsCollection: client.Database("projects_db").Collection("projects"),
		TasksCollection:    client.Database("tasks_db").Collection("tasks"),
	}
}

// CreateProject creates a new project with the specified parameters.
func (s *ProjectService) CreateProject(name string, description string, expectedEndDate time.Time, minMembers, maxMembers int, managerID primitive.ObjectID) (*models.Project, error) {
	// Validate input parameters
	if minMembers < 1 || maxMembers < minMembers {
		return nil, fmt.Errorf("invalid member constraints: minMembers=%d, maxMembers=%d", minMembers, maxMembers)
	}
	if expectedEndDate.Before(time.Now()) {
		return nil, fmt.Errorf("expected end date must be in the future")
	}

	// Create the project
	project := &models.Project{
		ID:              primitive.NewObjectID(),
		Name:            name,
		Description:     description,
		ExpectedEndDate: expectedEndDate,
		MinMembers:      minMembers,
		MaxMembers:      maxMembers,
		ManagerID:       managerID,
		Members:         []models.Member{},
		Tasks:           []models.Task{},
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

// GetProjectMembers retrieves members of a specific project.
func (s *ProjectService) GetProjectMembers(ctx context.Context, projectID string) ([]bson.M, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, err
	}

	var project struct {
		Members []bson.M `bson:"members"`
	}

	err = s.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		return nil, err
	}

	return project.Members, nil
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

	// Provera da li član ima aktivne zadatke
	taskFilter := bson.M{
		"projectId": projectObjectID.Hex(),
		"status":    "in progress",
		"assignees": memberID,
	}

	cursor, err := s.TasksCollection.Find(ctx, taskFilter)
	if err != nil {
		return errors.New("failed to check task assignments")
	}
	defer cursor.Close(ctx)

	var tasksInProgress []bson.M
	for cursor.Next(ctx) {
		var task bson.M
		if err := cursor.Decode(&task); err != nil {
			return errors.New("failed to decode task data")
		}
		tasksInProgress = append(tasksInProgress, task)
	}

	if len(tasksInProgress) > 0 {
		return errors.New("cannot remove member assigned to an in-progress task")
	}

	// Ažuriranje projekta kako bi se uklonio član iz liste
	projectFilter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"_id": memberObjectID}}}

	result, err := s.ProjectsCollection.UpdateOne(ctx, projectFilter, update)
	if err != nil {
		return errors.New("failed to remove member from project")
	}

	if result.ModifiedCount == 0 {
		return errors.New("member not found in project or already removed")
	}

	return nil
}
