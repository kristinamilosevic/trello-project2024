package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"trello-project/microservices/projects-service/models"
	"trello-project/microservices/projects-service/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ProjectHandler struct {
	service *services.ProjectService
}

// Kreirajte novi ProjectHandler
func NewProjectHandler(service *services.ProjectService) *ProjectHandler {
	return &ProjectHandler{service: service}
}

// Handler za kreiranje novog projekta
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var project models.Project
	if err := json.NewDecoder(r.Body).Decode(&project); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Postavi trenutnog korisnika kao menadžera projekta
	managerID := r.Header.Get("Manager-ID")
	if managerID == "" {
		http.Error(w, "Manager ID not provided", http.StatusUnauthorized)
		return
	}

	// Pretvori Manager-ID u ObjectID
	managerObjectID, err := primitive.ObjectIDFromHex(managerID)
	if err != nil {
		http.Error(w, "Invalid Manager ID", http.StatusBadRequest)
		return
	}
	project.ManagerID = managerObjectID

	// Proveri validnost atributa projekta prema specifikaciji
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

	// Kreiraj projekat koristeći servis
	createdProject, err := h.service.CreateProject(
		project.Name,
		project.ExpectedEndDate,
		project.MinMembers,
		project.MaxMembers,
		project.ManagerID,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Vrati uspešan odgovor sa kreiranim projektom
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdProject)
}
