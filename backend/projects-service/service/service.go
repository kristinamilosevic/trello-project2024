package service

import (
	"context"
	"trello-project/microservices/projects-service/model"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	Collection      *mongo.Collection
	UsersCollection *mongo.Collection
}

func NewProjectService(projectCollection, usersCollection *mongo.Collection) *ProjectService {
	return &ProjectService{
		Collection:      projectCollection,
		UsersCollection: usersCollection,
	}
}

func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, memberIDs []primitive.ObjectID) error {
	var members []model.Member

	// Dohvati podatke o korisnicima na osnovu ID-jeva iz `users` kolekcije
	for _, memberID := range memberIDs {
		var user model.Member // Pretpostavljamo da model `Member` sadrži polja koja odgovaraju korisnicima
		err := s.UsersCollection.FindOne(context.Background(), bson.M{"_id": memberID}).Decode(&user)
		if err != nil {
			return err // Greška ako član nije pronađen
		}
		members = append(members, user) // Dodajemo člana s istim ID-jem
	}

	// Ažuriranje `projects` kolekcije da sadrži nove članove
	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err := s.Collection.UpdateOne(context.Background(), filter, update)
	return err
}

func (s *ProjectService) GetProjectMembers(projectID primitive.ObjectID) ([]model.Member, error) {
	var project model.Project
	filter := bson.M{"_id": projectID}

	err := s.Collection.FindOne(context.Background(), filter).Decode(&project)
	if err != nil {
		return nil, err
	}

	return project.Members, nil
}

func (s *ProjectService) GetAllUsers() ([]model.Member, error) {
	var users []model.Member
	cursor, err := s.UsersCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &users); err != nil {
		return nil, err
	}
	return users, nil
}
