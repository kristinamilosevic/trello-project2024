package commands

import (
	"context"
	"fmt"
	"log"
	"trello-project/microservices/workflow-service/interfaces"
	"trello-project/microservices/workflow-service/models"
)

type AddDependencyCommand struct {
	Dependency models.TaskDependencyRelation
}

type AddDependencyHandler struct {
	GraphService interfaces.WorkflowCommandContext
}

func NewAddDependencyHandler(ctx interfaces.WorkflowCommandContext) *AddDependencyHandler {
	return &AddDependencyHandler{GraphService: ctx}
}

func (h *AddDependencyHandler) Handle(ctx context.Context, cmd AddDependencyCommand) error {
	err := h.GraphService.AddDependency(ctx, cmd.Dependency)
	if err != nil {
		return fmt.Errorf("failed to add dependency: %w", err)
	}

	// Nakon dodavanja zavisnosti, apdejtuj blokiranost taskova
	updateCmd := UpdateBlockedStatusCommand{
		TaskID: cmd.Dependency.ToTaskID,
		Svc:    h.GraphService,
	}
	if err := updateCmd.Execute(ctx); err != nil {
		log.Printf("warning: dependency added, but failed to update blocked statuses: %v", err)
	}

	return nil
}
