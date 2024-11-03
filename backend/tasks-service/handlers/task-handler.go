package handlers

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/tasks-service/models"   // Prilagodite putanju
	"trello-project/microservices/tasks-service/services" // Prilagodite putanju
)

type TaskHandler struct {
	service *services.TaskService
}

// Kreirajte novi TaskHandler
func NewTaskHandler(service *services.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

// Handler za kreiranje novog zadatka
func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	createdTask, err := h.service.CreateTask(task.ProjectID, task.Title, task.Description)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTask)
}
