package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in progress"
	StatusCompleted  TaskStatus = "completed"
)

type Task struct {
	ID          primitive.ObjectID  `json:"id" bson:"_id,omitempty"`
	ProjectID   string              `json:"projectId" bson:"projectId"`
	Title       string              `json:"title" bson:"title"`
	Description string              `json:"description" bson:"description"`
	Status      TaskStatus          `json:"status" bson:"status"`
	DependsOn   *primitive.ObjectID `json:"dependsOn,omitempty" bson:"dependsOn,omitempty"`
}
