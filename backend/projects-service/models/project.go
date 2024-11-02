package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Project struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	Members     []string           `bson:"members" json:"members"`
	TaskID      []string           `bson:"task_id" json:"task_id"`
}
