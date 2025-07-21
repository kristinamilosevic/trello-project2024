package interfaces

import (
	"context"
	"trello-project/microservices/workflow-service/models"
)

type Command interface {
	Execute() error
}

type Query interface {
	Execute() (interface{}, error)
}

type WorkflowCommandContext interface {
	AddDependency(ctx context.Context, dependency models.TaskDependencyRelation) error
	UpdateBlockedStatus(ctx context.Context, taskID string) error
	RemoveDependency(ctx context.Context, fromTaskID, toTaskID string) error
}

type WorkflowQueryContext interface {
	GetDependencies(ctx context.Context, taskId string) ([]models.TaskNode, error)
}
