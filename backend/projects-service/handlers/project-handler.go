package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"trello-project/microservices/projects-service/models"
	"trello-project/microservices/projects-service/services"
	"trello-project/microservices/projects-service/utils"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectHandler struct {
	Service *services.ProjectService
}

func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}
func checkRole(r *http.Request, allowedRoles []string) error {
	userRole := r.Header.Get("Role")
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
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		http.Error(w, "Authorization token required", http.StatusUnauthorized)
		return
	}

	// Ekstrakcija username-a iz tokena
	username, err := utils.ExtractManagerUsernameFromToken(strings.TrimPrefix(tokenString, "Bearer "))
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Pronađi ID menadžera u kolekciji korisnika koristeći username
	var manager struct {
		ID primitive.ObjectID `bson:"_id"`
	}

	err = h.Service.UsersCollection.FindOne(r.Context(), bson.M{"username": username}).Decode(&manager)
	if err != nil {
		http.Error(w, "Manager not found", http.StatusUnauthorized)
		return
	}

	managerID := manager.ID

	// Postavi ID menadžera u projekat
	var project models.Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	project.ManagerID = managerID

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
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProject)
}

func (h *ProjectHandler) AddMemberToProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

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
	err = h.Service.AddMembersToProject(projectID, memberIDs)
	if err != nil {
		switch err.Error() {
		case "all provided members are already part of the project":
			http.Error(w, "One or more members are already on the project", http.StatusBadRequest)
		case "maximum number of members reached for the project":
			http.Error(w, err.Error(), http.StatusBadRequest)
		case "you need to add at least the minimum required members to the project":
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "Failed to add members to project", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}

// GetProjectMembersHandler retrieves the members of a specified project
func (h *ProjectHandler) GetProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
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
	if err := checkRole(r, []string{"manager"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	fmt.Println("Request received:", r.URL.Path)

	vars := mux.Vars(r)
	projectID := vars["projectId"]
	memberID := vars["memberId"]

	fmt.Println("Extracted projectID:", projectID)
	fmt.Println("Extracted memberID:", memberID)

	err := h.Service.RemoveMemberFromProject(r.Context(), projectID, memberID)
	if err != nil {
		fmt.Println("Error during member removal:", err)
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
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

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
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
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

func (h *ProjectHandler) DisplayTasksForProjectHandler(w http.ResponseWriter, r *http.Request) {
	if err := checkRole(r, []string{"manager", "member"}); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	vars := mux.Vars(r)
	projectID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	tasks, err := h.Service.GetTasksForProject(projectID)
	if err != nil {
		if strings.Contains(err.Error(), "project not found") {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to retrieve tasks: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tasks)
}

func GetProjectsByUsername(s *services.ProjectService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := checkRole(r, []string{"manager", "member"}); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		vars := mux.Vars(r)
		username := vars["username"]
		if username == "" {
			http.Error(w, "Username is required", http.StatusBadRequest)
			return
		}

		log.Printf("Fetching projects for username: %s", username)

		projects, err := s.GetProjectsByUsername(username)
		if err != nil {
			log.Printf("Error fetching projects for username %s: %v", username, err)
			http.Error(w, fmt.Sprintf("Error fetching projects: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(projects); err != nil {
			log.Printf("Error encoding response for username %s: %v", username, err)
			http.Error(w, fmt.Sprintf("Error encoding response: %v", err), http.StatusInternalServerError)
		}
	}
}
