package handlers

import (
	"net/http"
	"strings"
	"trello-project/microservices/projects-service/services"
)

type ProjectHandler struct {
	Service *services.ProjectService
}

func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}

// Handler za uklanjanje člana iz projekta
func (h *ProjectHandler) RemoveMemberFromProjectHandler(w http.ResponseWriter, r *http.Request) {

	// /projects/{projectId}/members/{memberId}/remove
	pathParts := strings.Split(r.URL.Path, "/")

	if len(pathParts) < 6 {
		http.Error(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	// Pribavljanje projectID i memberID iz url
	projectID := pathParts[2]
	memberID := pathParts[4]

	err := h.Service.RemoveMemberFromProject(r.Context(), projectID, memberID)
	if err != nil {
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
