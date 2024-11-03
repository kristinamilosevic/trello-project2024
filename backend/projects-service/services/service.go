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

// Metoda za dobavljanje clanova odredjenog projekta
func (ps *ProjectService) GetProjectMembers(ctx context.Context, projectID string) ([]bson.M, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return nil, err
	}

	var project struct {
		Members []bson.M `bson:"members"`
	}

	err = ps.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		return nil, err
	}

	return project.Members, nil
}

func (ps *ProjectService) RemoveMemberFromProject(ctx context.Context, projectID, memberID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return errors.New("invalid project ID format")
	}

	fmt.Println("Checking tasks for member:", memberID, "in project:", projectObjectID.Hex())

	projectIDStr := projectObjectID.Hex()

	taskFilter := bson.M{
		"projectId": projectIDStr,
		"status":    "in progress",
		"assignees": memberID,
	}
	fmt.Println("Using filter:", taskFilter)

	cursor, err := ps.TasksCollection.Find(ctx, taskFilter)
	if err != nil {
		fmt.Println("Error finding tasks:", err)
		return errors.New("failed to check task assignments")
	}
	defer cursor.Close(ctx)

	// Provera listanja zadataka
	var tasksInProgress []bson.M
	for cursor.Next(ctx) {
		var task bson.M
		if err := cursor.Decode(&task); err != nil {
			fmt.Println("Error decoding task:", err)
			return errors.New("failed to decode task data")
		}
		tasksInProgress = append(tasksInProgress, task)
	}
	fmt.Printf("Tasks in progress for member %s: %v\n", memberID, tasksInProgress)

	if len(tasksInProgress) > 0 {
		return errors.New("cannot remove member assigned to an in-progress task")
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

	fmt.Println("Member removed successfully from project")
	return nil
}
