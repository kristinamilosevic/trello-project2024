package queries

import (
	"context"
	"trello-project/microservices/workflow-service/interfaces"
)

type GetDependenciesQuery struct {
	TaskID string
	Svc    interfaces.WorkflowQueryContext
}

func (q *GetDependenciesQuery) Execute() (interface{}, error) {
	ctx := context.Background()
	return q.Svc.GetDependencies(ctx, q.TaskID)
}
