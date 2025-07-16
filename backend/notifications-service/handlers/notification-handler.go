package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"notifications-service/logging"
	"notifications-service/services"
)

type NotificationHandler struct {
	service *services.NotificationService
}

// Konstruktor za NotificationHandler
func NewNotificationHandler(service *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		service: service,
	}
}
func checkRole(r *http.Request, allowedRoles []string) error {
	userRole := r.Header.Get("Role")
	logging.Logger.Debugf("Checking role: %s", userRole)
	if userRole == "" {
		logging.Logger.Warn("Role is missing in request header.")
		return fmt.Errorf("role is missing in request header")
	}

	// Proveri da li je uloga dozvoljena
	for _, role := range allowedRoles {
		if role == userRole {
			logging.Logger.Debugf("User role '%s' is allowed.", userRole)
			return nil
		}
	}
	logging.Logger.Warnf("Access forbidden: user role '%s' does not have the required role. Allowed roles: %v", userRole, allowedRoles)
	return fmt.Errorf("access forbidden: user does not have the required role")
}

// HTTP handler za kreiranje notifikacije
func (nh *NotificationHandler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Info("Attempting to create a new notification.")
	if err := checkRole(r, []string{"manager"}); err != nil {
		logging.Logger.Warnf("Access forbidden for CreateNotification: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	var req struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Invalid request payload for CreateNotification: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.UserID == "" || req.Username == "" || req.Message == "" {
		logging.Logger.Warnf("All fields (userId, username, message) are required for CreateNotification.")
		http.Error(w, "All fields (userId, username, message) are required", http.StatusBadRequest)
		return
	}

	if err := nh.service.CreateNotification(req.UserID, req.Username, req.Message); err != nil {
		logging.Logger.Errorf("Failed to create notification for user %s: %v", req.Username, err)
		http.Error(w, "Failed to create notification", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Notification successfully created for user %s (ID: %s)", req.Username, req.UserID)
	w.WriteHeader(http.StatusCreated)
}

// HTTP handler za dobijanje notifikacija korisnika
func (nh *NotificationHandler) GetNotificationsByUsername(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Info("Attempting to fetch notifications by username.")
	if err := checkRole(r, []string{"member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for GetNotificationsByUsername: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		logging.Logger.Warn("Missing username parameter for GetNotificationsByUsername.")
		http.Error(w, "Missing username parameter", http.StatusBadRequest)
		return
	}
	logging.Logger.Debugf("Fetching notifications for username: %s", username)
	notifications, err := nh.service.GetNotificationsByUsername(username)
	if err != nil {
		logging.Logger.Errorf("Failed to fetch notifications for username %s: %v", username, err)
		http.Error(w, "Failed to fetch notifications", http.StatusInternalServerError)
		return
	}

	// UVEK vrati JSON niz, čak i ako je prazan
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
	logging.Logger.Infof("Successfully fetched %d notifications for username %s.", len(notifications), username)
}

// HTTP handler za označavanje notifikacije kao pročitane
func (nh *NotificationHandler) MarkNotificationAsRead(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Infof("Attempting to mark notification as read.")
	var req struct {
		NotificationID string `json:"notificationId"`
		Username       string `json:"username"`
		CreatedAt      string `json:"createdAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Invalid request payload for MarkNotificationAsRead: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.NotificationID == "" || req.Username == "" || req.CreatedAt == "" {
		logging.Logger.Warnf("Notification ID, Username, and CreatedAt are required for MarkNotificationAsRead.")
		http.Error(w, "Notification ID, Username, and CreatedAt are required", http.StatusBadRequest)
		return
	}
	logging.Logger.Debugf("Marking notification %s for user %s as read.", req.NotificationID, req.Username)

	if err := nh.service.MarkNotificationAsRead(req.Username, req.NotificationID, req.CreatedAt); err != nil {
		logging.Logger.Errorf("Failed to mark notification %s as read for user %s: %v", req.NotificationID, req.Username, err)
		http.Error(w, "Failed to mark notification as read", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Notification %s successfully marked as read for user %s.", req.NotificationID, req.Username)
	w.WriteHeader(http.StatusOK)
}

// HTTP handler za brisanje notifikacije
func (nh *NotificationHandler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	logging.Logger.Infof("Attempting to delete a notification.")
	if err := checkRole(r, []string{"member"}); err != nil {
		logging.Logger.Warnf("Access forbidden for DeleteNotification: %v", err)
		http.Error(w, "Access forbidden: insufficient permissions", http.StatusForbidden)
		return
	}
	var req struct {
		NotificationID string `json:"notificationId"`
		Username       string `json:"username"`
		CreatedAt      string `json:"createdAt"`
	}

	// Dekodiraj JSON payload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logging.Logger.Warnf("Invalid request payload for DeleteNotification: %v", err)
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validacija
	if req.NotificationID == "" || req.Username == "" || req.CreatedAt == "" {
		logging.Logger.Warn("Notification ID, Username, and CreatedAt are required for DeleteNotification.")
		http.Error(w, "Notification ID, Username, and CreatedAt are required", http.StatusBadRequest)
		return
	}
	logging.Logger.Debugf("Deleting notification %s for user %s.", req.NotificationID, req.Username)
	// Poziv servisa
	if err := nh.service.DeleteNotification(req.Username, req.NotificationID, req.CreatedAt); err != nil {
		logging.Logger.Errorf("Failed to delete notification %s for user %s: %v", req.NotificationID, req.Username, err)
		http.Error(w, "Failed to delete notification", http.StatusInternalServerError)
		return
	}
	logging.Logger.Infof("Notification %s successfully deleted for user %s.", req.NotificationID, req.Username)
	w.WriteHeader(http.StatusOK)
}
