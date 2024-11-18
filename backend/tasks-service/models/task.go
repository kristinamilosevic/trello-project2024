package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TaskStatus string

const (
	StatusPending    TaskStatus = "Pending"
	StatusInProgress TaskStatus = "In progress"
	StatusCompleted  TaskStatus = "Completed"
)

type Task struct {
	ID          primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	ProjectID   string              `json:"projectId" bson:"projectId"`
	Title       string              `json:"title" bson:"title"`
	Description string              `json:"description" bson:"description"`
	Status      TaskStatus          `json:"status" bson:"status"`
	DependsOn   *primitive.ObjectID `json:"dependsOn,omitempty" bson:"dependsOn,omitempty"`
}
