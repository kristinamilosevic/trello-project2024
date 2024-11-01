package handler

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/projects-service/model"
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

	var members []model.Member
	if err := json.NewDecoder(r.Body).Decode(&members); err != nil {
		http.Error(w, "Invalid members data", http.StatusBadRequest)
		return
	}

	if err := h.Service.AddMembersToProject(projectID, members); err != nil {
		http.Error(w, "Failed to add members to project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "Members added successfully"}`))
}
