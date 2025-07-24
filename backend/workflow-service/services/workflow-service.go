package services

import (
	"context"
	"fmt"
	"trello-project/microservices/workflow-service/interfaces"
	"trello-project/microservices/workflow-service/logging"
	"trello-project/microservices/workflow-service/models"
	"trello-project/microservices/workflow-service/services/commands"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

type WorkflowService struct {
	Driver neo4j.DriverWithContext
}

func NewWorkflowService(driver neo4j.DriverWithContext) *WorkflowService {
	return &WorkflowService{Driver: driver}
}

func (s *WorkflowService) AddDependency(ctx context.Context, rel models.TaskDependencyRelation) error {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	logging.Logger.Infof("Attempting to add dependency: %s <- %s", rel.ToTaskID, rel.FromTaskID)

	exist, err := s.TasksExist(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		logging.Logger.Errorf("Failed to check if tasks exist: %v", err)
		return fmt.Errorf("failed to check task existence: %v", err)
	}
	if !exist {
		logging.Logger.Warnf("One or both tasks do not exist: from=%s, to=%s", rel.FromTaskID, rel.ToTaskID)
		return fmt.Errorf("one or both tasks do not exist")
	}

	exists, err := s.DependencyExists(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		logging.Logger.Errorf("Failed to check if dependency exists: %v", err)
		return fmt.Errorf("failed to check if dependency exists: %v", err)
	}
	if exists {
		logging.Logger.Warnf("Dependency already exists: %s <- %s", rel.ToTaskID, rel.FromTaskID)
		return fmt.Errorf("dependency already exists")
	}

	hasCycle, err := s.CreatesCycle(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		logging.Logger.Errorf("Failed to check for cycle: %v", err)
		return fmt.Errorf("failed to check cycle: %v", err)
	}
	if hasCycle {
		logging.Logger.Warnf("Cycle detected when trying to add dependency: %s <- %s", rel.ToTaskID, rel.FromTaskID)
		return fmt.Errorf("cannot add dependency: cycle detected")
	}

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (from:Task {id: $fromId}), (to:Task {id: $toId})
			MERGE (to)-[:DEPENDS_ON]->(from)
			SET to.blocked = true
		`
		_, err := tx.Run(ctx, query, map[string]any{
			"fromId": rel.FromTaskID,
			"toId":   rel.ToTaskID,
		})
		return nil, err
	})

	if err != nil {
		logging.Logger.Errorf("Failed to create dependency relation: %v", err)
		return fmt.Errorf("failed to create dependency relation: %v", err)
	}

	logging.Logger.Infof("Dependency successfully added: %s <- %s", rel.ToTaskID, rel.FromTaskID)

	cmd := commands.UpdateBlockedStatusCommand{
		TaskID: rel.ToTaskID,
		Svc:    interfaces.WorkflowCommandContext(s),
	}
	if err := cmd.Execute(ctx); err != nil {
		logging.Logger.Warnf("Failed to update blocked status for task %s: %v", rel.ToTaskID, err)
	}

	return nil
}

func (s *WorkflowService) CreatesCycle(ctx context.Context, fromID, toID string) (bool, error) {
	if fromID == toID {
		logging.Logger.Warnf("Cycle detected: task cannot depend on itself (id=%s)", fromID)
		return true, nil
	}

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	logging.Logger.Infof("Checking for cycle: %s -> %s", fromID, toID)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (from:Task {id: $fromId}), (to:Task {id: $toId})
			RETURN EXISTS((from)-[:DEPENDS_ON*1..]->(to)) AS hasCycle
		`
		res, err := tx.Run(ctx, query, map[string]any{
			"fromId": fromID,
			"toId":   toID,
		})
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			val, ok := res.Record().Values[0].(bool)
			if !ok {
				logging.Logger.Errorf("Unexpected result type during cycle check")
				return false, fmt.Errorf("unexpected result type")
			}
			return val, nil
		}
		return false, nil
	})

	if err != nil {
		logging.Logger.Errorf("Cycle detection query failed: %v", err)
		return false, fmt.Errorf("cycle detection failed: %v", err)
	}

	logging.Logger.Infof("Cycle check result for %s -> %s: %v", fromID, toID, result.(bool))
	return result.(bool), nil
}

func (s *WorkflowService) TasksExist(ctx context.Context, id1, id2 string) (bool, error) {
	logging.Logger.Infof("Checking if both tasks exist: id1=%s, id2=%s", id1, id2)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			OPTIONAL MATCH (a:Task {id: $id1})
			OPTIONAL MATCH (b:Task {id: $id2})
			RETURN a IS NOT NULL AND b IS NOT NULL AS bothExist
		`
		res, err := tx.Run(ctx, query, map[string]any{
			"id1": id1,
			"id2": id2,
		})
		if err != nil {
			return false, err
		}
		if res.Next(ctx) {
			return res.Record().Values[0].(bool), nil
		}
		return false, nil
	})

	if err != nil {
		logging.Logger.Errorf("Error checking task existence: %v", err)
		return false, err
	}

	logging.Logger.Infof("TasksExist result for %s and %s: %v", id1, id2, result.(bool))
	return result.(bool), nil
}

func (s *WorkflowService) EnsureTaskNode(ctx context.Context, task models.TaskNode) error {
	logging.Logger.Infof("Ensuring task node: %+v", task)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MERGE (t:Task {id: $id})
			ON CREATE SET 
				t.projectId = $projectId,
				t.name = $name,
				t.description = $description,
				t.blocked = $blocked
		`
		params := map[string]any{
			"id":          task.ID,
			"projectId":   task.ProjectID,
			"name":        task.Name,
			"description": task.Description,
			"blocked":     task.Blocked,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	if err != nil {
		logging.Logger.Errorf("Failed to ensure task node %s: %v", task.ID, err)
	} else {
		logging.Logger.Infof("Task node ensured: %s", task.ID)
	}

	return err
}

func (s *WorkflowService) GetDependencies(ctx context.Context, taskId string) ([]models.TaskNode, error) {
	logging.Logger.Infof("Fetching dependencies for task: %s", taskId)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (to:Task {id: $taskId})-[:DEPENDS_ON]->(from:Task)
			RETURN from.id AS id, from.projectId AS projectId, from.name AS name,
			       from.description AS description, from.blocked AS blocked
		`
		res, err := tx.Run(ctx, query, map[string]any{"taskId": taskId})
		if err != nil {
			return nil, err
		}

		var dependencies []models.TaskNode
		for res.Next(ctx) {
			record := res.Record()

			id, _ := record.Get("id")
			projectId, _ := record.Get("projectId")
			name, _ := record.Get("name")
			description, _ := record.Get("description")
			blocked, _ := record.Get("blocked")

			task := models.TaskNode{
				ID:          id.(string),
				ProjectID:   projectId.(string),
				Name:        name.(string),
				Description: description.(string),
				Blocked:     blocked.(bool),
			}
			dependencies = append(dependencies, task)
		}

		return dependencies, nil
	})

	if err != nil {
		logging.Logger.Errorf("Failed to get dependencies for task %s: %v", taskId, err)
		return nil, err
	}

	logging.Logger.Infof("Fetched %d dependencies for task %s", len(result.([]models.TaskNode)), taskId)
	return result.([]models.TaskNode), nil
}

func (s *WorkflowService) DependencyExists(ctx context.Context, fromID, toID string) (bool, error) {
	logging.Logger.Infof("Checking if dependency exists: %s <- %s", toID, fromID)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (to:Task {id: $toId})-[r:DEPENDS_ON]->(from:Task {id: $fromId})
			RETURN COUNT(r) > 0 AS exists
		`
		res, err := tx.Run(ctx, query, map[string]any{
			"fromId": fromID,
			"toId":   toID,
		})
		if err != nil {
			return false, err
		}
		if res.Next(ctx) {
			return res.Record().Values[0].(bool), nil
		}
		return false, nil
	})

	if err != nil {
		logging.Logger.Errorf("Error checking dependency existence for %s <- %s: %v", toID, fromID, err)
		return false, err
	}

	logging.Logger.Infof("Dependency exists result for %s <- %s: %v", toID, fromID, result.(bool))
	return result.(bool), nil
}

func (s *WorkflowService) UpdateBlockedStatus(ctx context.Context, taskID string) error {
	logging.Logger.Infof("Updating blocked status for task: %s", taskID)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	dependencies, err := s.GetDependencies(ctx, taskID)
	if err != nil {
		logging.Logger.Errorf("Failed to fetch dependencies for task %s: %v", taskID, err)
		return fmt.Errorf("failed to fetch dependencies: %v", err)
	}

	isBlocked := len(dependencies) > 0

	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (t:Task {id: $taskId})
			SET t.blocked = $isBlocked
		`
		_, err := tx.Run(ctx, query, map[string]any{
			"taskId":    taskID,
			"isBlocked": isBlocked,
		})
		return nil, err
	})

	if err != nil {
		logging.Logger.Errorf("Failed to update blocked status for task %s: %v", taskID, err)
		return fmt.Errorf("failed to update blocked status: %v", err)
	}

	logging.Logger.Infof("Blocked status for task %s updated to %v", taskID, isBlocked)
	return nil
}

func (s *WorkflowService) SetBlockedStatus(ctx context.Context, taskID string, blocked bool) error {
	logging.Logger.Infof("Setting blocked status for task %s to %v", taskID, blocked)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	query := `
        MATCH (t:Task {id: $taskID})
        SET t.blocked = $blocked
    `
	params := map[string]interface{}{
		"taskID":  taskID,
		"blocked": blocked,
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	if err != nil {
		logging.Logger.Errorf("Failed to set blocked status for task %s: %v", taskID, err)
		return fmt.Errorf("failed to set blocked status in db: %w", err)
	}

	logging.Logger.Infof("Blocked status for task %s successfully set to %v", taskID, blocked)
	return nil
}

func (s *WorkflowService) GetProjectDependencies(ctx context.Context, projectID string) ([]models.TaskDependencyRelation, error) {
	logging.Logger.Infof("Fetching project dependencies for project: %s", projectID)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := ` 
			MATCH (to:Task {projectId: $projectId})-[:DEPENDS_ON]->(from:Task)
            RETURN from.id AS fromTaskId, to.id AS toTaskId
		`

		res, err := tx.Run(ctx, query, map[string]interface{}{"projectId": projectID})
		if err != nil {
			return nil, err
		}

		var deps []models.TaskDependencyRelation
		for res.Next(ctx) {
			record := res.Record()
			deps = append(deps, models.TaskDependencyRelation{
				FromTaskID: record.Values[0].(string),
				ToTaskID:   record.Values[1].(string),
			})
		}
		return deps, nil
	})

	if err != nil {
		logging.Logger.Errorf("Failed to fetch project dependencies for project %s: %v", projectID, err)
		return nil, err
	}

	logging.Logger.Infof("Fetched %d project dependencies for project %s", len(result.([]models.TaskDependencyRelation)), projectID)
	return result.([]models.TaskDependencyRelation), nil
}

func (s *WorkflowService) GetWorkflowByProject(ctx context.Context, projectID string) ([]models.TaskNode, []models.TaskDependencyRelation, error) {
	logging.Logger.Infof("Fetching workflow graph for project: %s", projectID)

	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	nodes, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (t:Task {projectId: $projectId})
			RETURN t.id AS id, t.projectId AS projectId, t.name AS name,
			       t.description AS description, t.blocked AS blocked
		`
		res, err := tx.Run(ctx, query, map[string]any{"projectId": projectID})
		if err != nil {
			return nil, err
		}

		var taskNodes []models.TaskNode
		for res.Next(ctx) {
			record := res.Record()

			id, _ := record.Get("id")
			projectId, _ := record.Get("projectId")
			name, _ := record.Get("name")
			description, _ := record.Get("description")
			blocked, _ := record.Get("blocked")

			taskNodes = append(taskNodes, models.TaskNode{
				ID:          id.(string),
				ProjectID:   projectId.(string),
				Name:        name.(string),
				Description: description.(string),
				Blocked:     blocked.(bool),
			})
		}

		return taskNodes, nil
	})
	if err != nil {
		logging.Logger.Errorf("Failed to fetch task nodes for project %s: %v", projectID, err)
		return nil, nil, fmt.Errorf("failed to fetch task nodes: %w", err)
	}

	dependencies, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (to:Task {projectId: $projectId})-[:DEPENDS_ON]->(from:Task)
			RETURN from.id AS fromId, to.id AS toId
		`
		res, err := tx.Run(ctx, query, map[string]any{"projectId": projectID})
		if err != nil {
			return nil, err
		}

		var relations []models.TaskDependencyRelation
		for res.Next(ctx) {
			record := res.Record()

			fromID, _ := record.Get("fromId")
			toID, _ := record.Get("toId")

			relations = append(relations, models.TaskDependencyRelation{
				FromTaskID: fromID.(string),
				ToTaskID:   toID.(string),
			})
		}

		return relations, nil
	})
	if err != nil {
		logging.Logger.Errorf("Failed to fetch dependency relations for project %s: %v", projectID, err)
		return nil, nil, fmt.Errorf("failed to fetch dependency relations: %w", err)
	}

	logging.Logger.Infof("Workflow graph fetched for project %s: %d tasks, %d dependencies", projectID, len(nodes.([]models.TaskNode)), len(dependencies.([]models.TaskDependencyRelation)))
	return nodes.([]models.TaskNode), dependencies.([]models.TaskDependencyRelation), nil
}
