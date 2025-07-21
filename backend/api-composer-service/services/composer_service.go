package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"trello-project/microservices/api-composer-service/models"
)

func FetchGraphData(projectID, authHeader, roleHeader string) (models.GraphResponse, error) {
	client := &http.Client{}

	//Poziv tasks-service
	tasksReq, err := http.NewRequest("GET", fmt.Sprintf("http://tasks-service:8002/api/tasks/project/%s", projectID), nil)
	if err != nil {
		return models.GraphResponse{}, fmt.Errorf("error creating request to tasks-service: %v", err)
	}
	if authHeader != "" {
		tasksReq.Header.Set("Authorization", authHeader)
	}
	if roleHeader != "" {
		tasksReq.Header.Set("role", roleHeader)
	}

	tasksResp, err := client.Do(tasksReq)
	if err != nil {
		return models.GraphResponse{}, fmt.Errorf("error sending request to tasks-service: %v", err)
	}
	defer tasksResp.Body.Close()

	if tasksResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tasksResp.Body)
		return models.GraphResponse{}, fmt.Errorf("tasks-service error (%d): %s", tasksResp.StatusCode, string(body))
	}

	var tasks []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(tasksResp.Body).Decode(&tasks); err != nil {
		return models.GraphResponse{}, fmt.Errorf("failed to decode tasks-service response: %v", err)
	}

	//Poziv workflow-service
	workflowReq, err := http.NewRequest("GET", fmt.Sprintf("http://workflow-service:8005/api/workflow/project/%s/dependencies", projectID), nil)

	if err != nil {
		return models.GraphResponse{}, fmt.Errorf("error creating request to workflow-service: %v", err)
	}
	if authHeader != "" {
		workflowReq.Header.Set("Authorization", authHeader)
	}
	if roleHeader != "" {
		workflowReq.Header.Set("role", roleHeader)
	}

	workflowsResp, err := client.Do(workflowReq)
	if err != nil {
		return models.GraphResponse{}, fmt.Errorf("error sending request to workflow-service: %v", err)
	}
	defer workflowsResp.Body.Close()

	if workflowsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(workflowsResp.Body)
		return models.GraphResponse{}, fmt.Errorf("workflow-service error (%d): %s", workflowsResp.StatusCode, string(body))
	}

	var edges []struct {
		From string `json:"fromTaskId"`
		To   string `json:"toTaskId"`
	}
	if err := json.NewDecoder(workflowsResp.Body).Decode(&edges); err != nil {
		return models.GraphResponse{}, fmt.Errorf("failed to decode workflow-service response: %v", err)
	}

	//Formiranje grafa
	var graph models.GraphResponse
	for _, t := range tasks {
		graph.Nodes = append(graph.Nodes, models.GraphNode{
			ID:          t.ID,
			Title:       t.Title,
			Description: t.Description,
		})
	}
	for _, e := range edges {
		graph.Edges = append(graph.Edges, models.GraphEdge{
			From: e.From,
			To:   e.To,
		})
	}

	return graph, nil
}
