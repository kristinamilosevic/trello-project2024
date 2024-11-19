package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Task struct {
	ID          primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ProjectID   string             `json:"projectId" bson:"projectId"`
	Title       string             `json:"title" bson:"title"`
	Description string             `json:"description" bson:"description"`
	Status      string             `json:"status" bson:"status"`
	Members     []Member           `json:"members" bson:"members"`
	Assignees   []Member           `bson:"assignees" json:"assignees"`
}
