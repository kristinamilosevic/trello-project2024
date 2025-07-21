package handlers

import (
	"encoding/json"
	"net/http"
	"trello-project/microservices/api-composer-service/services"

	"github.com/gorilla/mux"
)

func GetGraphHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectId"]

	authHeader := r.Header.Get("Authorization")
	roleHeader := r.Header.Get("role")

	graph, err := services.FetchGraphData(projectID, authHeader, roleHeader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(graph)
}
