package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"trello-project/microservices/projects-service/logging"
	"trello-project/microservices/projects-service/models"
	"trello-project/microservices/projects-service/services"
	"trello-project/microservices/projects-service/utils"

	"github.com/gorilla/mux"
	// "go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectHandler struct {
	Service            *services.ProjectService
	ProjectsCollection *mongo.Collection
}

func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}

func checkRole(r *http.Request, allowedRoles []string) error {
	userRole := r.Header.Get("Role")
	logging.Logger.Debugf("Checking role: %s", userRole)
	if userRole == "" {
		return fmt.Errorf("role is missing in request header")
	}

	// Proveri da li je uloga dozvoljena
	for _, role := range allowedRoles {
		if role == userRole {
			return nil
		}
	}
	return fmt.Errorf("access forbidden: user does not have the required role")
}

func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for CreateProject: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		logging.Logger.Warn("Authorization token required for CreateProject")
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	username, err := utils.ExtractManagerUsernameFromToken(strings.TrimPrefix(tokenString, "Bearer "))
	if err != nil {
		logging.Logger.Errorf("Failed to extract manager username from token: %v", err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	logging.Logger.Infof("Attempting to create project by manager: %s", username)

	userServiceURL := os.Getenv("USERS_SERVICE_URL")
	url := fmt.Sprintf("%s/api/users/member/%s", userServiceURL, username)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.Logger.Errorf("Failed to create user service request: %v", err)
		http.Error(w, "Failed to create user service request", http.StatusInternalServerError)
		return
	}

	req.Header.Set("Authorization", tokenString)

	resp, err := h.Service.HTTPClient.Do(req)
	if err != nil {
		logging.Logger.Errorf("Failed to contact users service: %v", err)
		http.Error(w, "Failed to contact users service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Logger.Warnf("Manager not found in users service, status code: %d", resp.StatusCode)
		http.Error(w, "Manager not found in users service", http.StatusUnauthorized)
		return
	}

	var userResp struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		logging.Logger.Errorf("Failed to parse users service response: %v", err)
		http.Error(w, "Failed to parse users service response", http.StatusInternalServerError)
		return
	}

	managerID, err := primitive.ObjectIDFromHex(userResp.ID)
	if err != nil {
		logging.Logger.Errorf("Invalid manager ID format: %v", err)
		http.Error(w, "Invalid manager ID format", http.StatusInternalServerError)
		return
	}

	var project models.Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		logging.Logger.Warnf("Invalid request payload for CreateProject: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	project.ManagerID = managerID

	if project.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}
	if project.ExpectedEndDate.Before(time.Now()) {
		logging.Logger.Warn("Project name is required")
		http.Error(w, "Expected end date must be in the future", http.StatusBadRequest)
		return
	}
	if project.MinMembers < 1 || project.MaxMembers < project.MinMembers {
		logging.Logger.Warnf("Invalid member constraints: MinMembers=%d, MaxMembers=%d", project.MinMembers, project.MaxMembers)
		http.Error(w, "Invalid member constraints", http.StatusBadRequest)
		return
	}

	createdProject, err := h.Service.CreateProject(
		project.Name,
		project.Description,
		project.ExpectedEndDate,
		project.MinMembers,
		project.MaxMembers,
		project.ManagerID,
	)
	if err != nil {
		if err.Error() == "project with the same name already exists" {
			logging.Logger.Warnf("Project creation conflict: %v", err)
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		logging.Logger.Errorf("Failed to create project: %v", err)
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Project '%s' created successfully by manager %s", createdProject.Name, username)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProject)
}

func (h *ProjectHandler) AddMemberToProjectHandler(w http.ResponseWriter, r *http.Request) {
	// Provera da li je korisnik menadžer
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for AddMemberToProjectHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// Dobavljanje projectID iz URL-a
	vars := mux.Vars(r)
	projectID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		logging.Logger.Warnf("Invalid project ID for AddMemberToProjectHandler: %v", err)
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}
	logging.Logger.Infof("Attempting to add members to project ID: %s", projectID.Hex())

	// Parsiranje JSON zahteva
	var request struct {
		Usernames []string `json:"usernames"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logging.Logger.Warnf("Invalid members data payload for AddMemberToProjectHandler: %v", err)
		http.Error(w, "Invalid members data", http.StatusBadRequest)
		return
	}

	// Provera da li postoji bar jedan username za dodavanje
	if len(request.Usernames) == 0 {
		logging.Logger.Warn("No usernames provided for AddMemberToProjectHandler")
		http.Error(w, "No usernames provided", http.StatusBadRequest)
		return
	}

	// Poziv servisa za dodavanje članova pomoću username-a
	err = h.Service.AddMembersToProject(projectID, request.Usernames)
	if err != nil {
		switch err.Error() {
		case "all provided members are already part of the project":
			logging.Logger.Warnf("One or more members already on project %s: %v", projectID.Hex(), err)
			http.Error(w, "One or more members are already on the project", http.StatusBadRequest)
		case "maximum number of members reached for the project":
			logging.Logger.Warnf("Maximum members reached for project %s: %v", projectID.Hex(), err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		case "you need to add at least the minimum required members to the project":
			logging.Logger.Warnf("Minimum required members not met for project %s: %v", projectID.Hex(), err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			logging.Logger.Errorf("Failed to add members to project %s: %v", projectID.Hex(), err)
			http.Error(w, "Failed to add members to project", http.StatusInternalServerError)
		}
		return
	}

	logging.Logger.Infof("Members %v added successfully to project %s", request.Usernames, projectID.Hex())
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}

// GetProjectMembersHandler retrieves the members of a specified project
func (h *ProjectHandler) GetProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for GetProjectMembersHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	projectID := vars["projectId"]

	logging.Logger.Debugf("Fetching members for project: %s", projectID)
	members, err := h.Service.GetProjectMembers(r.Context(), projectID)
	if err != nil {
		logging.Logger.Errorf("Error in service GetProjectMembers for project %s: %v", projectID, err)
		http.Error(w, "Failed to retrieve members", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Successfully retrieved members for project %s", projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// RemoveMemberFromProjectHandler removes a member from a project if they have no in-progress tasks
func (h *ProjectHandler) RemoveMemberFromProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for RemoveMemberFromProjectHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	logging.Logger.Debugf("Request received to remove member: %s", r.URL.Path)

	vars := mux.Vars(r)
	projectID := vars["projectId"]
	memberID := vars["memberId"]

	logging.Logger.Debugf("Extracted projectID: %s, memberID: %s", projectID, memberID)

	err := h.Service.RemoveMemberFromProject(r.Context(), projectID, memberID)
	if err != nil {
		logging.Logger.Errorf("Error during member removal from project %s, member %s: %v", projectID, memberID, err)
		if err.Error() == "cannot remove member assigned to an in-progress task" {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else if err.Error() == "project not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Failed to remove member from project", http.StatusInternalServerError)
		}
		return
	}

	logging.Logger.Infof("Member %s removed successfully from project %s", memberID, projectID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Member removed successfully from project"}`))
}

// GetAllUsersHandler retrieves all users
func (h *ProjectHandler) GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Infof("Attempting to retrieve all users.")
	users, err := h.Service.GetAllUsers()
	if err != nil {
		logging.Logger.Errorf("Failed to retrieve users: %v", err)
		http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Successfully retrieved all users.")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// ListProjectsHandler - dobavlja sve projekte
func (h *ProjectHandler) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for ListProjectsHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	logging.Logger.Debug("Fetching all projects...")

	projects, err := h.Service.GetAllProjects()
	if err != nil {
		logging.Logger.Errorf("Error fetching projects from service: %v", err)
		http.Error(w, "Error fetching projects", http.StatusInternalServerError)
		return
	}

	logging.Logger.Debugf("Projects fetched successfully. Count: %d", len(projects))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

// GetProjectByIDHandler - Dohvata projekat po ID-ju
func (h *ProjectHandler) GetProjectByIDHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for GetProjectByIDHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	projectID := vars["id"]
	logging.Logger.Debugf("Fetching project by ID: %s", projectID)

	project, err := h.Service.GetProjectByID(projectID)
	if err != nil {
		if err.Error() == "project not found" {
			logging.Logger.Warnf("Project not found for ID: %s", projectID)
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			logging.Logger.Errorf("Error fetching project by ID %s: %v", projectID, err)
			http.Error(w, "Error fetching project", http.StatusInternalServerError)
		}
		return
	}

	logging.Logger.Infof("Successfully retrieved project by ID: %s", projectID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

func (h *ProjectHandler) DisplayTasksForProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for DisplayTasksForProjectHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	projectID := vars["id"]
	if projectID == "" {
		logging.Logger.Warn("Invalid project ID provided for DisplayTasksForProjectHandler")
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}
	logging.Logger.Debugf("Displaying tasks for project ID: %s", projectID)

	role := r.Header.Get("Role")
	authToken := r.Header.Get("Authorization")

	tasks, err := h.Service.GetTasksForProject(projectID, role, authToken)
	if err != nil {
		if strings.Contains(err.Error(), "project not found") {
			logging.Logger.Warnf("Project not found for displaying tasks: %s", projectID)
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		logging.Logger.Errorf("Failed to retrieve tasks for project %s: %v", projectID, err)
		http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Successfully retrieved tasks for project ID: %s", projectID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tasks)
}

func GetProjectsByUsername(s *services.ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logging.Logger.Debug("Received request for GetProjectsByUsername")
		if err := checkRole(r, []string{"manager", "member"}); err != nil {
			logging.Logger.Warnf("Access forbidden for GetProjectsByUsername: %v", err)
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		vars := mux.Vars(r)
		username := vars["username"]
		if username == "" {
			logging.Logger.Warn("Username is required for GetProjectsByUsername")
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}

		logging.Logger.Infof("Fetching projects for username: %s", username)

		projects, err := s.GetProjectsByUsername(username)
		if err != nil {
			logging.Logger.Errorf("Error fetching projects for username %s: %v", username, err)
			http.Error(w, fmt.Sprintf("Error fetching projects: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(projects); err != nil {
			logging.Logger.Errorf("Error encoding response for username %s: %v", username, err)
			http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		}
		logging.Logger.Infof("Successfully retrieved projects for username: %s", username)
	}
}

func (h *ProjectHandler) RemoveProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for RemoveProjectHandler: insufficient permissions. Error: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]

	logging.Logger.Infof("Received request to delete project with ID: %s", projectID)

	err := h.Service.DeleteProjectAndTasks(r.Context(), projectID, r)
	if err != nil {
		logging.Logger.Errorf("Failed to delete project and tasks for ID %s: %v", projectID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Project and related tasks deleted successfully for ID: %s", projectID)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Project and related tasks deleted successfully"})
}

func (h *ProjectHandler) GetAllMembersHandler(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Info("Attempting to retrieve all members.")
	members, err := h.Service.GetAllMembers()
	if err != nil {
		logging.Logger.Errorf("Failed to fetch members: %v", err)
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}

	logging.Logger.Info("Successfully retrieved all members.")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(members)
}

func (h *ProjectHandler) AddTaskToProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for AddTaskToProjectHandler: %v", err)
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]
	logging.Logger.Infof("Attempting to add task to project ID: %s", projectID)

	var request struct {
		TaskID string `json:"taskID"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		logging.Logger.Warnf("Invalid request payload for AddTaskToProjectHandler: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if request.TaskID == "" {
		logging.Logger.Warn("TaskID is required for AddTaskToProjectHandler")
		http.Error(w, "TaskID is required", http.StatusBadRequest)
		return
	}

	err := h.Service.AddTaskToProject(projectID, request.TaskID)
	if err != nil {
		logging.Logger.Errorf("Failed to add task %s to project %s: %v", request.TaskID, projectID, err)
		http.Error(w, fmt.Sprintf("Failed to add task to project: %v", err), http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Task %s added to project %s successfully", request.TaskID, projectID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Task added to project successfully"}`))
}

func (h *ProjectHandler) GetUserProjectsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userID"]

	logging.Logger.Infof("Fetching projects for user ID: %s", userID)

	projects, err := h.Service.GetUserProjects(userID)
	if err != nil {
		logging.Logger.Errorf("Error fetching projects for user %s: %v", userID, err)
		http.Error(w, "Error fetching projects", http.StatusInternalServerError)
		return
	}

	logging.Logger.Infof("Successfully retrieved projects for user ID: %s", userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

func (h *ProjectHandler) RemoveUserFromProjectsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["userID"]
	role := r.URL.Query().Get("role")

	if userID == "" || role == "" {
		logging.Logger.Warn("userID and role are required for RemoveUserFromProjectsHandler")
		http.Error(w, "userID and role are required", http.StatusBadRequest)
		return
	}
	logging.Logger.Infof("Attempting to remove user %s (role: %s) from all projects.", userID, role)

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		logging.Logger.Warn("Missing Authorization header for RemoveUserFromProjectsHandler")
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	authToken := strings.TrimPrefix(authHeader, "Bearer ")
	if authToken == "" {
		logging.Logger.Warn("Invalid Authorization header format for RemoveUserFromProjectsHandler")
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	err := h.Service.RemoveUserFromProjects(userID, role, authToken)
	if err != nil {
		logging.Logger.Errorf("Failed to remove user %s from projects: %v", userID, err)
		http.Error(w, fmt.Sprintf("Failed to remove user from projects: %v", err), http.StatusBadRequest)
		return
	}

	logging.Logger.Infof("User %s successfully removed from all projects", userID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User successfully removed from all projects"))
}
