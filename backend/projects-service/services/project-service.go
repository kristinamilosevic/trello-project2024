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

// AddMembersToProject adds multiple members to a project.
func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, memberIDs []primitive.ObjectID) error {
	var members []models.Member

	// Fetch user data from the users collection
	for _, memberID := range memberIDs {
		var user models.Member
		err := s.UsersCollection.FindOne(context.Background(), bson.M{"_id": memberID}).Decode(&user)
		if err != nil {
			return err // Error if member is not found
		}
		members = append(members, user)
	}

	// Update the project collection to add the new members
	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err := s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	return err
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

	// Check if the member has any active tasks
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

	// Update the project to remove the member from the list
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
