package commands

import (
	"context"
	"log"
	"trello-project/microservices/workflow-service/interfaces"
)

type RemoveDependencyCommand struct {
	FromTaskID string
	ToTaskID   string
}

type RemoveDependencyHandler struct {
	GraphService interfaces.WorkflowCommandContext // koristi interfejs, ne konkretan tip
}

func NewRemoveDependencyHandler(ctx interfaces.WorkflowCommandContext) *RemoveDependencyHandler {
	return &RemoveDependencyHandler{GraphService: ctx}
}

func (h *RemoveDependencyHandler) Handle(ctx context.Context, cmd RemoveDependencyCommand) error {

	log.Printf("[RemoveDependencyHandler] Removing dependency from %s to %s", cmd.FromTaskID, cmd.ToTaskID)

	err := h.GraphService.RemoveDependency(ctx, cmd.FromTaskID, cmd.ToTaskID)
	if err != nil {
		return err
	}

	updateCmd := UpdateBlockedStatusCommand{
		TaskID: cmd.ToTaskID,
		Svc:    h.GraphService,
	}
	return updateCmd.Execute(ctx)

}
