package services

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	ProjectsCollection *mongo.Collection
	TasksCollection    *mongo.Collection
}

func (ps *ProjectService) RemoveMemberFromProject(ctx context.Context, projectID, memberID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return errors.New("invalid project ID format")
	}

	// Konvertujemo projectID u string za upit u tasks kolekciji
	projectIDStr := projectObjectID.Hex()

	// Proveravamo da li član ima neki zadatak u statusu "in progress"
	taskFilter := bson.M{
		"projectId": projectIDStr,
		"status":    "in progress",
		"assignees": memberID,
	}
	fmt.Println("Checking for tasks with filter:", taskFilter)

	// Dohvatamo listu zadataka sa statusom "in progress" za zadatog člana
	var tasksInProgress []bson.M
	cursor, err := ps.TasksCollection.Find(ctx, taskFilter)
	if err != nil {
		fmt.Println("Error finding tasks:", err)
		return errors.New("failed to check task assignments")
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var task bson.M
		if err := cursor.Decode(&task); err != nil {
			fmt.Println("Error decoding task:", err)
			return errors.New("failed to decode task data")
		}
		tasksInProgress = append(tasksInProgress, task)
	}
	fmt.Printf("Tasks in progress for member %s: %v\n", memberID, tasksInProgress)

	// Ako postoji barem jedan takav zadatak, član ne može biti uklonjen
	if len(tasksInProgress) > 0 {
		return errors.New("cannot remove member assigned to an in-progress task")
	}

	var projectBefore struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Members []struct {
			ID   string `bson:"id"`
			Name string `bson:"name"`
		} `bson:"members"`
	}
	err = ps.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&projectBefore)
	if err != nil {
		return errors.New("failed to retrieve project data before update")
	}

	projectFilter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"id": memberID}}}

	result, err := ps.ProjectsCollection.UpdateOne(ctx, projectFilter, update)
	if err != nil {
		fmt.Println("Error during UpdateOne:", err)
		return errors.New("failed to remove member from project")
	}

	if result.ModifiedCount == 0 {
		return errors.New("member not found in project or already removed")
	}

	return nil
}
