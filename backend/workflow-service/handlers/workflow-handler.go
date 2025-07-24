package handlers

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/workflow-service/logging"
	"trello-project/microservices/workflow-service/models"
	"trello-project/microservices/workflow-service/services"
	"trello-project/microservices/workflow-service/services/commands"

	"github.com/gorilla/mux"
)

type WorkflowHandler struct {
	WorkflowService *services.WorkflowService
}

// /komentar
func NewWorkflowHandler(service *services.WorkflowService) *WorkflowHandler {
	return &WorkflowHandler{
		WorkflowService: service,
	}
}

func (h *WorkflowHandler) AddDependency(w http.ResponseWriter, r *http.Request) {
	var relation models.TaskDependencyRelation

	logging.Logger.Infof("Received AddDependency request")

	if err := json.NewDecoder(r.Body).Decode(&relation); err != nil {
		logging.Logger.Errorf("Failed to decode request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if relation.FromTaskID == "" || relation.ToTaskID == "" {
		logging.Logger.Warn("Missing task IDs in dependency relation")
		http.Error(w, "Missing task IDs", http.StatusBadRequest)
		return
	}

	handler := commands.NewAddDependencyHandler(h.WorkflowService)
	cmd := commands.AddDependencyCommand{Dependency: relation}
	err := handler.Handle(r.Context(), cmd)

	if err != nil {
		logging.Logger.Errorf("Failed to add dependency: %v", err)
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

	logging.Logger.Infof("Dependency added successfully: %s <- %s", relation.ToTaskID, relation.FromTaskID)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Dependency successfully added"))
}

func (h *WorkflowHandler) EnsureTaskNode(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Info("Received EnsureTaskNode request")

	var taskNode models.TaskNode
	if err := json.NewDecoder(r.Body).Decode(&taskNode); err != nil {
		logging.Logger.Errorf("Failed to decode task node: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.WorkflowService.EnsureTaskNode(r.Context(), taskNode)
	if err != nil {
		logging.Logger.Errorf("Failed to ensure task node: %v", err)
		http.Error(w, "Failed to ensure task node: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Task node ensured: %s", taskNode.ID)
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Task node ensured"))
}

func (h *WorkflowHandler) GetDependencies(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskId := vars["taskId"]

	logging.Logger.Infof("Received GetDependencies request for task: %s", taskId)

	if taskId == "" {
		logging.Logger.Warn("Missing taskId parameter")
		http.Error(w, "Missing taskId parameter", http.StatusBadRequest)
		return
	}

	deps, err := h.WorkflowService.GetDependencies(r.Context(), taskId)
	if err != nil {
		logging.Logger.Errorf("Failed to get dependencies: %v", err)
		http.Error(w, "Failed to get dependencies: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Dependencies fetched for task %s: count = %d", taskId, len(deps))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deps)
}

func (h *WorkflowHandler) GetProjectDependencies(w http.ResponseWriter, r *http.Request) {
	projectId := mux.Vars(r)["projectId"]

	logging.Logger.Infof("Received GetProjectDependencies request for project: %s", projectId)

	if projectId == "" {
		logging.Logger.Warn("Missing projectId parameter")
		http.Error(w, "Missing projectId", http.StatusBadRequest)
		return
	}

	deps, err := h.WorkflowService.GetProjectDependencies(r.Context(), projectId)
	if err != nil {
		logging.Logger.Errorf("Failed to get project dependencies: %v", err)
		http.Error(w, "Failed to get dependencies: "+err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Project dependencies fetched for project %s: count = %d", projectId, len(deps))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deps)
}

func (h *WorkflowHandler) GetWorkflowGraph(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["projectId"]

	logging.Logger.Infof("Received GetWorkflowGraph request for project: %s", projectID)

	nodes, dependencies, err := h.WorkflowService.GetWorkflowByProject(r.Context(), projectID)
	if err != nil {
		logging.Logger.Errorf("Failed to get workflow graph for project %s: %v", projectID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Workflow graph loaded for project %s: nodes=%d, dependencies=%d", projectID, len(nodes), len(dependencies))

	response := map[string]interface{}{
		"nodes":        nodes,
		"dependencies": dependencies,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
