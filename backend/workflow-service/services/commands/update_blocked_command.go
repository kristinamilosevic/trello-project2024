package commands

import (
	"context"
	"trello-project/microservices/workflow-service/interfaces"
)

type UpdateBlockedStatusCommand struct {
	TaskID string
	Svc    interfaces.WorkflowCommandContext
}

func (cmd *UpdateBlockedStatusCommand) Execute(ctx context.Context) error {
	return cmd.Svc.UpdateBlockedStatus(ctx, cmd.TaskID)
}
