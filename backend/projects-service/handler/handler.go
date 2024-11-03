package handler

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/projects-service/service"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectHandler struct {
	Service *service.ProjectService
}

func NewProjectHandler(service *service.ProjectService) *ProjectHandler {
	return &ProjectHandler{Service: service}
}

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

	if err := h.Service.AddMembersToProject(projectID, memberIDs); err != nil {
		http.Error(w, "Failed to add members to project", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}

func (h *ProjectHandler) GetProjectMembersHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID, err := primitive.ObjectIDFromHex(vars["id"])
	if err != nil {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	members, err := h.Service.GetProjectMembers(projectID)
	if err != nil {
		http.Error(w, "Failed to retrieve members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *ProjectHandler) GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	users, err := h.Service.GetAllUsers()
	if err != nil {
		http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
