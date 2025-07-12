package services

import (
	"context"
	"fmt"
	"log"
	"trello-project/microservices/workflow-service/models"

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

	// Prvo proverimo da li oba taska postoje u grafu
	exist, err := s.TasksExist(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		return fmt.Errorf("failed to check task existence: %v", err)
	}
	if !exist {
		return fmt.Errorf("one or both tasks do not exist")
	}

	// 🔸 Proveri da li veza već postoji
	exists, err := s.DependencyExists(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		return fmt.Errorf("failed to check if dependency exists: %v", err)
	}
	if exists {
		return fmt.Errorf("dependency already exists")
	}

	// Proveri da li bi novo povezivanje uzrokovalo ciklus
	hasCycle, err := s.CreatesCycle(ctx, rel.FromTaskID, rel.ToTaskID)
	if err != nil {
		return fmt.Errorf("failed to check cycle: %v", err)
	}
	if hasCycle {
		return fmt.Errorf("cannot add dependency: cycle detected")
	}

	// Dodavanje relacije ako je sve prošlo
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
		return fmt.Errorf("failed to create dependency relation: %v", err)
	}

	log.Printf("Dependency added: %s <- %s", rel.ToTaskID, rel.FromTaskID)
	return nil
}

func (s *WorkflowService) CreatesCycle(ctx context.Context, fromID, toID string) (bool, error) {
	if fromID == toID {
		return true, nil
	}
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

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
				return false, fmt.Errorf("unexpected result type")
			}
			return val, nil
		}
		return false, nil
	})

	if err != nil {
		return false, fmt.Errorf("cycle detection failed: %v", err)
	}

	return result.(bool), nil
}

func (s *WorkflowService) TasksExist(ctx context.Context, id1, id2 string) (bool, error) {
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
		return false, err
	}

	return result.(bool), nil
}

func (s *WorkflowService) EnsureTaskNode(ctx context.Context, task models.TaskNode) error {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MERGE (t:Task {id: $id})
			ON CREATE SET 
				t.projectId = $projectId,
				t.name = $name,
				t.description = $description,
				t.status = $status,
				t.blocked = $blocked
		`
		params := map[string]any{
			"id":          task.ID,
			"projectId":   task.ProjectID,
			"name":        task.Name,
			"description": task.Description,
			"status":      task.Status,
			"blocked":     task.Blocked,
		}
		_, err := tx.Run(ctx, query, params)
		return nil, err
	})

	return err
}

func (s *WorkflowService) GetDependencies(ctx context.Context, taskId string) ([]models.TaskNode, error) {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		query := `
			MATCH (to:Task {id: $taskId})-[:DEPENDS_ON]->(from:Task)
			RETURN from.id AS id, from.projectId AS projectId, from.name AS name,
			       from.description AS description, from.status AS status, from.blocked AS blocked
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
			status, _ := record.Get("status")
			blocked, _ := record.Get("blocked")

			task := models.TaskNode{
				ID:          id.(string),
				ProjectID:   projectId.(string),
				Name:        name.(string),
				Description: description.(string),
				Status:      status.(string),
				Blocked:     blocked.(bool),
			}
			dependencies = append(dependencies, task)
		}

		return dependencies, nil
	})

	if err != nil {
		return nil, err
	}

	return result.([]models.TaskNode), nil
}

func (s *WorkflowService) DependencyExists(ctx context.Context, fromID, toID string) (bool, error) {
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
		return false, err
	}
	return result.(bool), nil
}

func (s *WorkflowService) UpdateBlockedStatus(ctx context.Context, taskID string) error {
	session := s.Driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(ctx)

	// 1. Dohvati sve zavisnosti (taskovi od kojih zavisi ovaj task)
	dependencies, err := s.GetDependencies(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to fetch dependencies: %v", err)
	}

	// 2. Ako nema zavisnosti — nije blokiran
	isBlocked := false
	for _, dep := range dependencies {
		if dep.Status != "In progress" && dep.Status != "Completed" {
			isBlocked = true
			break
		}
	}

	// 3. Ažuriraj blokiran status u grafu
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
		return fmt.Errorf("failed to update blocked status: %v", err)
	}

	log.Printf("Blocked status for task %s updated to %v", taskID, isBlocked)
	return nil
}
