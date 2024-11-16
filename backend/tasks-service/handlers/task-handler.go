package handlers

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/tasks-service/models"
	"trello-project/microservices/tasks-service/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaskHandler struct {
	service *services.TaskService
}

func NewTaskHandler(service *services.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Ako status nije naveden, postavi ga na "pending"
	if task.Status == "" {
		task.Status = models.StatusPending
	}

	// Sada prosleđujemo status prilikom kreiranja taska
	createdTask, err := h.service.CreateTask(task.ProjectID, task.Title, task.Description, task.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTask)
}

func (h *TaskHandler) GetAllTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.GetAllTasks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tasks)
}

// promena statusa
func (h *TaskHandler) ChangeTaskStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TaskID   string            `json:"taskId"`
		Status   models.TaskStatus `json:"status"`
		Username string            `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	taskObjectID, err := primitive.ObjectIDFromHex(request.TaskID)
	if err != nil {
		http.Error(w, "Invalid task ID format", http.StatusBadRequest)
		return
	}

	updatedTask, err := h.service.ChangeTaskStatus(taskObjectID, request.Status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedTask)
}
