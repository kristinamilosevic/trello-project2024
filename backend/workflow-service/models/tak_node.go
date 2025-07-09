package models

type TaskNode struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Blocked     bool   `json:"blocked"`
}
