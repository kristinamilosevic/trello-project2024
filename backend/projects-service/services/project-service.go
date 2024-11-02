package services

import (
	"context"
	"fmt"
	"time"
	"trello-project/microservices/projects-service/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	collection *mongo.Collection
}

// Kreirajte novi ProjectService
func NewProjectService(client *mongo.Client) *ProjectService {
	return &ProjectService{collection: client.Database("projects_db").Collection("projects")}
}

// Funkcija za kreiranje novog projekta
func (s *ProjectService) CreateProject(name string, expectedEndDate time.Time, minMembers, maxMembers int, managerID primitive.ObjectID) (*models.Project, error) {
	// Validacija ulaznih parametara (po specifikaciji)
	if minMembers < 1 || maxMembers < minMembers {
		return nil, fmt.Errorf("invalid member constraints: minMembers=%d, maxMembers=%d", minMembers, maxMembers)
	}
	if expectedEndDate.Before(time.Now()) {
		return nil, fmt.Errorf("expected end date must be in the future")
	}

	// Kreiranje projekta
	project := &models.Project{
		ID:              primitive.NewObjectID(),
		Name:            name,
		ExpectedEndDate: expectedEndDate,
		MinMembers:      minMembers,
		MaxMembers:      maxMembers,
		ManagerID:       managerID,
	}

	// Umetanje projekta u kolekciju
	result, err := s.collection.InsertOne(context.Background(), project)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %v", err)
	}

	// Postavljanje ID-a projekta
	project.ID = result.InsertedID.(primitive.ObjectID)
	return project, nil
}
