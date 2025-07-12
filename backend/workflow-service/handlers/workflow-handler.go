package handlers

import (
	"context"
	"encoding/json"
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

	// Validacija ID-jeva
	if relation.FromTaskID == "" || relation.ToTaskID == "" {
		http.Error(w, "Missing task IDs", http.StatusBadRequest)
		return
	}

	// Poziv servisa za dodavanje zavisnosti
	err := h.WorkflowService.AddDependency(context.Background(), relation)
	if err != nil {
		if err.Error() == "dependency already exists" {
			http.Error(w, "Dependency already exists", http.StatusConflict) // 409
			return
		}
		if err.Error() == "cannot add dependency: cycle detected" {
			http.Error(w, "Cannot add dependency due to cycle", http.StatusConflict) // 409
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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
