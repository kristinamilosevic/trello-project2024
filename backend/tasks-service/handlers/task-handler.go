package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"trello-project/microservices/tasks-service/logging"
	"trello-project/microservices/tasks-service/models"
	"trello-project/microservices/tasks-service/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TaskHandler struct {
	service *services.TaskService
}

func NewTaskHandler(service *services.TaskService) *TaskHandler {
	return &TaskHandler{service: service}
}
func checkRole(r *http.Request, allowedRoles []string) error {
	userRole := r.Header.Get("Role")
	if userRole == "" {
		logging.Logger.Warnf("Event ID: AUTH_MISSING_ROLE, Description: Role header is missing in request from %s for path %s", r.RemoteAddr, r.URL.Path)
		return fmt.Errorf("role is missing in request header")
	}

	// Proveri da li je uloga dozvoljena
	for _, role := range allowedRoles {
		if role == userRole {
			return nil
		}
	}
	logging.Logger.Warnf("Event ID: AUTH_FORBIDDEN_ROLE, Description: Access forbidden for user with role '%s' to path %s. Required roles: %v", userRole, r.URL.Path, allowedRoles)
	return fmt.Errorf("access forbidden: user does not have the required role")
}

func (h *TaskHandler) CreateTask(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	var task models.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		logging.Logger.Errorf("Event ID: TASK_CREATE_DECODE_ERROR, Description: Failed to decode request body for task creation: %v", err)
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
		logging.Logger.Errorf("Event ID: TASK_CREATE_SERVICE_ERROR, Description: Failed to create task in service: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: TASK_CREATED_SUCCESS, Description: Task '%s' created successfully for Project ID '%s'. Task ID: %s", createdTask.Title, createdTask.ProjectID, createdTask.ID.Hex())
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdTask)
}

func (h *TaskHandler) GetAllTasks(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	tasks, err := h.service.GetAllTasks()
	if err != nil {
		logging.Logger.Errorf("Event ID: TASKS_GET_ALL_SERVICE_ERROR, Description: Failed to retrieve all tasks from service: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Event ID: TASKS_GET_ALL_SUCCESS, Description: Successfully retrieved %d tasks.", len(tasks))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tasks)
}
func (h *TaskHandler) GetAvailableMembersForTask(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	taskID := vars["taskID"]
	projectID := vars["projectID"]

	logging.Logger.Debugf("Event ID: MEMBERS_AVAILABLE_REQUEST, Description: Request to get available members for taskID: %s, projectID: %s", taskID, projectID)

	// Ako nedostaju taskID ili projectID, vraćamo grešku
	if taskID == "" || projectID == "" {
		logging.Logger.Warnf("Event ID: MEMBERS_AVAILABLE_BAD_REQUEST, Description: Missing taskID or projectID in request for available members.")
		http.Error(w, "taskID or projectID is required", http.StatusBadRequest)
		return
	}

	// Pozivamo servis za dobijanje dostupnih članova - sada prosleđujemo i `r`
	members, err := h.service.GetAvailableMembersForTask(r, projectID, taskID)
	if err != nil {
		logging.Logger.Errorf("Event ID: MEMBERS_AVAILABLE_SERVICE_ERROR, Description: Failed to get available members from service for taskID %s, projectID %s: %v", taskID, projectID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: MEMBERS_AVAILABLE_SUCCESS, Description: Successfully retrieved %d available members for taskID %s, projectID %s.", len(members), taskID, projectID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(members)
}

func (h TaskHandler) GetTasksByProjectID(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}

	// Loguj celu URL putanju
	logging.Logger.Debugf("Event ID: TASKS_BY_PROJECT_REQUEST, Description: Requested URL: %s", r.URL.Path)

	// Ekstraktuj projectID
	projectID := strings.TrimPrefix(r.URL.Path, "/api/tasks/project/")
	logging.Logger.Debugf("Event ID: TASKS_BY_PROJECT_EXTRACTED_ID, Description: Extracted Project ID: %s", projectID)

	// Ukloni eventualne nepotrebne '/' karaktere
	projectID = strings.Trim(projectID, "/")

	// Ako je projectID prazan, vrati grešku
	if projectID == "" {
		logging.Logger.Warnf("Event ID: TASKS_BY_PROJECT_MISSING_ID, Description: Project ID is missing or empty in request.")
		http.Error(w, "Missing project ID", http.StatusBadRequest)
		return
	}

	tasks, err := h.service.GetTasksByProjectID(projectID)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASKS_BY_PROJECT_SERVICE_ERROR, Description: Error fetching tasks for project ID %s: %v", projectID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Loguj broj pronađenih zadataka
	logging.Logger.Infof("Event ID: TASKS_BY_PROJECT_SUCCESS, Description: Found %d tasks for project ID: %s", len(tasks), projectID)

	// Vrati rezultate
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tasks)
}

func (h *TaskHandler) AddMembersToTask(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	logging.Logger.Debugf("Event ID: TASK_ADD_MEMBERS_REQUEST, Description: Task ID received for adding members: %s", taskID)

	var members []models.Member
	if err := json.NewDecoder(r.Body).Decode(&members); err != nil {
		logging.Logger.Errorf("Event ID: TASK_ADD_MEMBERS_DECODE_ERROR, Description: Error decoding request body for adding members to task %s: %v", taskID, err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	logging.Logger.Debugf("Event ID: TASK_ADD_MEMBERS_PAYLOAD, Description: Members received for task %s: %+v", taskID, members)

	err := h.service.AddMembersToTask(taskID, members)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_ADD_MEMBERS_SERVICE_ERROR, Description: Error adding members to task %s: %v", taskID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: TASK_ADD_MEMBERS_SUCCESS, Description: Members added successfully to task %s.", taskID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}

// GetMembersForTaskHandler dohvaća članove dodeljene određenom tasku
func (h *TaskHandler) GetMembersForTaskHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	taskID := vars["taskID"]

	// Konverzija taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_GET_MEMBERS_INVALID_ID, Description: Invalid task ID format for getting members: %v", err)
		http.Error(w, "Invalid task ID format", http.StatusBadRequest)
		return
	}

	// Pozovi servis za dobijanje članova zadatka
	members, err := h.service.GetMembersForTask(taskObjectID)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_GET_MEMBERS_SERVICE_ERROR, Description: Error fetching members for task %s: %v", taskID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: TASK_GET_MEMBERS_SUCCESS, Description: Successfully retrieved %d members for task %s.", len(members), taskID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// promena statusa
func (h TaskHandler) ChangeTaskStatus(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"member"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}

	var request struct {
		TaskID   string            `json:"taskId"`
		Status   models.TaskStatus `json:"status"`
		Username string            `json:"username"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logging.Logger.Errorf("Event ID: TASK_CHANGE_STATUS_DECODE_ERROR, Description: Invalid request payload for changing task status: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	taskObjectID, err := primitive.ObjectIDFromHex(request.TaskID)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_CHANGE_STATUS_INVALID_ID, Description: Invalid task ID format for changing status: %v", err)
		http.Error(w, "Invalid task ID format", http.StatusBadRequest)
		return
	}

	updatedTask, err := h.service.ChangeTaskStatus(taskObjectID, request.Status, request.Username)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_CHANGE_STATUS_SERVICE_ERROR, Description: Failed to change task status for task %s to %s by user %s: %v", request.TaskID, request.Status, request.Username, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	logging.Logger.Infof("Event ID: TASK_STATUS_CHANGED_SUCCESS, Description: Status of task '%s' successfully updated to: %s by user %s.", updatedTask.Title, updatedTask.Status, request.Username)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedTask)
}

// RemoveMemberFromTaskHandler uklanja člana sa zadatka
func (h *TaskHandler) RemoveMemberFromTaskHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	taskID := vars["taskID"]
	memberIDStr := vars["memberID"]

	// Konvertuj memberID u ObjectID
	memberID, err := primitive.ObjectIDFromHex(memberIDStr)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_REMOVE_MEMBER_INVALID_MEMBER_ID, Description: Invalid member ID format for removing from task %s: %v", taskID, err)
		http.Error(w, "Invalid member ID format", http.StatusBadRequest)
		return
	}

	// Pozivamo servis za uklanjanje člana
	err = h.service.RemoveMemberFromTask(taskID, memberID)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_REMOVE_MEMBER_SERVICE_ERROR, Description: Failed to remove member %s from task %s: %v", memberIDStr, taskID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: TASK_MEMBER_REMOVED_SUCCESS, Description: Member %s removed successfully from task %s.", memberIDStr, taskID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Member removed from task successfully and notification sent"}`))
}
func (h *TaskHandler) DeleteTasksByProjectHandler(w http.ResponseWriter, r *http.Request) {
	// Provera uloge korisnika
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}

	// Ekstrakcija projectId iz URL-a
	vars := mux.Vars(r)
	projectID := vars["projectId"]

	// Pozivanje servisa za brisanje zadataka
	err := h.service.DeleteTasksByProject(projectID)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASKS_DELETE_BY_PROJECT_SERVICE_ERROR, Description: Failed to delete tasks for project ID %s: %v", projectID, err)
		http.Error(w, "Failed to delete tasks", http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: TASKS_DELETED_BY_PROJECT_SUCCESS, Description: Successfully deleted tasks for project ID %s.", projectID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Tasks deleted successfully"})
}

func (h *TaskHandler) HasActiveTasksHandler(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	memberID := r.URL.Query().Get("memberId")

	if projectID == "" || memberID == "" {
		logging.Logger.Warnf("Event ID: HAS_ACTIVE_TASKS_BAD_REQUEST, Description: Missing projectId or memberId in request.")
		http.Error(w, "Missing projectId or memberId", http.StatusBadRequest)
		return
	}

	hasActive, err := h.service.HasActiveTasks(r.Context(), projectID, memberID)
	if err != nil {
		logging.Logger.Errorf("Event ID: HAS_ACTIVE_TASKS_SERVICE_ERROR, Description: Error checking active tasks for project %s, member %s: %v", projectID, memberID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: HAS_ACTIVE_TASKS_RESULT, Description: Checked active tasks for project %s, member %s. Result: %t", projectID, memberID, hasActive)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"hasActiveTasks": hasActive})
}
func (h *TaskHandler) HasUnfinishedTasksHandler(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["projectId"]
	if projectID == "" {
		logging.Logger.Warnf("Event ID: HAS_UNFINISHED_TASKS_BAD_REQUEST, Description: projectId is required for checking unfinished tasks.")
		http.Error(w, "projectId is required", http.StatusBadRequest)
		return
	}

	tasks, err := h.service.GetTasksByProjectID(projectID)
	if err != nil {
		logging.Logger.Errorf("Event ID: HAS_UNFINISHED_TASKS_SERVICE_ERROR, Description: Failed to get tasks for project %s to check unfinished tasks: %v", projectID, err)
		http.Error(w, "failed to get tasks: "+err.Error(), http.StatusInternalServerError)
		return
	}

	hasUnfinished := HasUnfinishedTasks(tasks)

	logging.Logger.Infof("Event ID: HAS_UNFINISHED_TASKS_RESULT, Description: Checked unfinished tasks for project %s. Result: %t", projectID, hasUnfinished)
	resp := map[string]bool{"hasUnfinishedTasks": hasUnfinished}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func HasUnfinishedTasks(tasks []models.Task) bool {
	for _, task := range tasks {
		if task.Status != models.StatusCompleted {
			return true
		}
	}
	return false
}

func (h *TaskHandler) RemoveUserFromAllTasksByUsername(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	username := vars["username"]

	err := h.service.RemoveUserFromAllTasksByUsername(username)
	if err != nil {
		logging.Logger.Errorf("Event ID: USER_REMOVE_FROM_ALL_TASKS_SERVICE_ERROR, Description: Failed to remove user '%s' from all tasks: %v", username, err)
		http.Error(w, fmt.Sprintf("Failed to remove user from tasks: %v", err), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Event ID: USER_REMOVED_FROM_ALL_TASKS_SUCCESS, Description: User '%s' removed from all tasks successfully.", username)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User removed from all tasks successfully"))
}
