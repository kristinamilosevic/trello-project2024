package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"trello-project/microservices/projects-service/services"

	"github.com/gorilla/mux"
)

type ProjectHandler struct {
	Service *services.ProjectService
}

func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}

func (h *ProjectHandler) GetProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectId"]

	// Pozivamo servis za dobavljanje clanova odredjenog projekta
	members, err := h.Service.GetProjectMembers(r.Context(), projectID)
	if err != nil {
		http.Error(w, "Failed to fetch project members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

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
