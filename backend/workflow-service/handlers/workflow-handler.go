package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"trello-project/microservices/workflow-service/models"
	"trello-project/microservices/workflow-service/services"

	"github.com/gorilla/mux"
)

type WorkflowHandler struct {
	WorkflowService *services.WorkflowService
}

func NewWorkflowHandler(service *services.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{
		WorkflowService: service,
	}
}

func (h *WorkflowHandler) AddDependency(w http.ResponseWriter, r *http.Request) {
	var relation models.TaskDependencyRelation

	if err := json.NewDecoder(r.Body).Decode(&relation); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if relation.FromTaskID == "" || relation.ToTaskID == "" {
		http.Error(w, "Missing task IDs", http.StatusBadRequest)
		return
	}

	err := h.WorkflowService.AddDependency(context.Background(), relation)
	if err != nil {
		msg := err.Error()
		switch msg {
		case "dependency already exists":
			http.Error(w, "Dependency already exists", http.StatusConflict)
			return
		case "cannot add dependency: cycle detected":
			http.Error(w, "Cannot add dependency due to cycle", http.StatusConflict)
			return
		default:
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Dependency successfully added"))
}

func (h *WorkflowHandler) EnsureTaskNode(w http.ResponseWriter, r *http.Request) {
	var taskNode models.TaskNode
	if err := json.NewDecoder(r.Body).Decode(&taskNode); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.WorkflowService.EnsureTaskNode(r.Context(), taskNode)
	if err != nil {
		http.Error(w, "Failed to ensure task node: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Task node ensured"))
}

func (h *WorkflowHandler) GetDependencies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskId := vars["taskId"]

	if taskId == "" {
		http.Error(w, "Missing taskId parameter", http.StatusBadRequest)
		return
	}

	deps, err := h.WorkflowService.GetDependencies(r.Context(), taskId)
	if err != nil {
		http.Error(w, "Failed to get dependencies: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deps)
}

func (h *WorkflowHandler) UpdateBlockedStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskId := vars["taskId"]

	var req struct {
		Blocked bool `json:"blocked"`
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.WorkflowService.SetBlockedStatus(taskId, req.Blocked)
	if err != nil {
		log.Printf("Failed to update blocked status for task %s: %v", taskId, err)
		http.Error(w, "Failed to update blocked status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Blocked status updated successfully"))
}

func (h *WorkflowHandler) GetWorkflowGraph(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["projectId"]

	nodes, dependencies, err := h.WorkflowService.GetWorkflowByProject(r.Context(), projectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"nodes":        nodes,
		"dependencies": dependencies,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
