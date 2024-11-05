package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"trello-project/microservices/projects-service/models"
	"trello-project/microservices/projects-service/services"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectHandler struct {
	Service *services.ProjectService
}

// NewProjectHandler creates a new ProjectHandler
func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}

// CreateProject handles the creation of a new project
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var project models.Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Set the manager as the current user based on the "Manager-ID" header
	managerID := r.Header.Get("Manager-ID")
	if managerID == "" {
		http.Error(w, "Manager ID not provided", http.StatusUnauthorized)
		return
	}

	// Convert Manager-ID to ObjectID
	managerObjectID, err := primitive.ObjectIDFromHex(managerID)
	if err != nil {
		http.Error(w, "Invalid Manager ID", http.StatusBadRequest)
		return
	}
	project.ManagerID = managerObjectID

	// Validate project attributes
	if project.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}
	if project.ExpectedEndDate.Before(time.Now()) {
		http.Error(w, "Expected end date must be in the future", http.StatusBadRequest)
		return
	}
	if project.MinMembers < 1 || project.MaxMembers < project.MinMembers {
		http.Error(w, "Invalid member constraints", http.StatusBadRequest)
		return
	}

	// Create project using the service
	createdProject, err := h.Service.CreateProject(
		project.Name,
		project.Description,
		project.ExpectedEndDate,
		project.MinMembers,
		project.MaxMembers,
		project.ManagerID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response with created project
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProject)
}

// AddMemberToProjectHandler adds multiple members to a project
func (h *ProjectHandler) AddMemberToProjectHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	var memberIDs []primitive.ObjectID
	if err := json.NewDecoder(r.Body).Decode(&memberIDs); err != nil {
		http.Error(w, "Invalid members data", http.StatusBadRequest)
		return
	}

	// Pozivamo servis za dodavanje članova i proveravamo greške
	if err := h.Service.AddMembersToProject(projectID, memberIDs); err != nil {
		if err.Error() == "dostignut je maksimalan broj članova za projekat" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Failed to add members to project", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}

// GetProjectMembersHandler retrieves the members of a specified project
func (h *ProjectHandler) GetProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectId"]

	fmt.Println("Fetching members for project:", projectID) // Log za proveren format ID-ja

	members, err := h.Service.GetProjectMembers(r.Context(), projectID)
	if err != nil {
		fmt.Println("Error in service GetProjectMembers:", err) // Log za grešku
		http.Error(w, "Failed to retrieve members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

// RemoveMemberFromProjectHandler removes a member from a project if they have no in-progress tasks
func (h *ProjectHandler) RemoveMemberFromProjectHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) != 6 || pathParts[3] != "members" || pathParts[5] != "remove" {
		http.NotFound(w, r)
		return
	}

	projectID := pathParts[2]
	memberID := pathParts[4]

	fmt.Println("Attempting to remove member:", memberID, "from project:", projectID)

	err := h.Service.RemoveMemberFromProject(r.Context(), projectID, memberID)
	if err != nil {
		fmt.Println("Error removing member:", err)
		if err.Error() == "cannot remove member assigned to an in-progress task" {
			http.Error(w, err.Error(), http.StatusForbidden)
		} else if err.Error() == "project not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Failed to remove member from project", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Member removed successfully from project"}`))
}

// GetAllUsersHandler retrieves all users
func (h *ProjectHandler) GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.Service.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

// ListProjectsHandler - dobavlja sve projekte
func (h *ProjectHandler) ListProjectsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Fetching all projects...") // Log za praćenje

	projects, err := h.Service.GetAllProjects()
	if err != nil {
		fmt.Println("Error fetching projects from service:", err) // Log za grešku
		http.Error(w, "Error fetching projects", http.StatusInternalServerError)
		return
	}

	fmt.Println("Projects fetched successfully:", projects) // Log za uspešan odziv

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects)
}

// GetProjectByIDHandler - Dohvata projekat po ID-ju
func (h *ProjectHandler) GetProjectByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["id"]

	project, err := h.Service.GetProjectByID(projectID)
	if err != nil {
		if err.Error() == "project not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching project", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}
