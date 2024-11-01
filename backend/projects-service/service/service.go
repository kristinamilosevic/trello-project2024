package service

import (
	"context"
	"trello-project/microservices/projects-service/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	Collection *mongo.Collection
}

func NewProjectService(collection *mongo.Collection) *ProjectService {
	return &ProjectService{Collection: collection}
}

func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, members []model.Member) error {
	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err := s.Collection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		return err
	}
	return nil
}
