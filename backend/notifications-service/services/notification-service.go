package services

import (
	"fmt"
	"notifications-service/logging"
	"notifications-service/models"
	"notifications-service/repositories"
	"time"
)

type NotificationService struct {
	repo *repositories.NotificationRepo
}

// Konstruktor za NotificationService
func NewNotificationService(repo *repositories.NotificationRepo) *NotificationService {
	return &NotificationService{
		repo: repo,
	}
}

// Kreiranje nove notifikacije
func (ns *NotificationService) CreateNotification(userID, username, message string) error {
	logging.Logger.Infof("Service: Attempting to create notification for user %s (ID: %s)", username, userID)
	if userID == "" || username == "" || message == "" {
		err := fmt.Errorf("userID, username, and message are required")
		logging.Logger.Warnf("Service: Validation failed for CreateNotification: %v", err)
		return fmt.Errorf("userID, username, and message are required")
	}
	notification := models.Notification{
		ID:        "", // ID će biti generisan u repozitorijumu
		UserID:    userID,
		Username:  username,
		Message:   message,
		CreatedAt: time.Now(),
		IsRead:    false,
	}
	if err := ns.repo.CreateNotification(&notification); err != nil {
		logging.Logger.Errorf("Service: Failed to create notification in repository for user %s: %v", username, err)
		return err
	}
	logging.Logger.Infof("Service: Notification successfully prepared and sent to repository for user %s", username)
	return nil
}

// Dohvatanje svih notifikacija za korisnika
func (ns *NotificationService) GetNotificationsByUsername(username string) ([]models.Notification, error) {
	logging.Logger.Infof("Service: Fetching notifications for username: %s", username)
	if username == "" {
		err := fmt.Errorf("username is required")
		logging.Logger.Warnf("Service: Validation failed for GetNotificationsByUsername: %v", err)
		return nil, err
	}
	notifications, err := ns.repo.GetNotificationsByUsername(username)
	if err != nil {
		logging.Logger.Errorf("Service: Failed to fetch notifications from repository for username %s: %v", username, err)
		return nil, err
	}
	logging.Logger.Infof("Service: Successfully fetched %d notifications from repository for username: %s", len(notifications), username)
	return notifications, nil
}

// Ažuriranje notifikacije (označavanje kao pročitano)
func (ns *NotificationService) MarkNotificationAsRead(username, notificationID, createdAt string) error {
	logging.Logger.Infof("Service: Attempting to mark notification %s as read for user %s", notificationID, username)
	if username == "" || notificationID == "" || createdAt == "" {
		err := fmt.Errorf("username, notificationID, and createdAt are required")
		logging.Logger.Warnf("Service: Validation failed for MarkNotificationAsRead: %v", err)
		return err
	}

	if err := ns.repo.MarkNotificationAsRead(username, notificationID, createdAt); err != nil {
		logging.Logger.Errorf("Service: Failed to mark notification %s as read in repository for user %s: %v", notificationID, username, err)
		return err
	}
	logging.Logger.Infof("Service: Notification %s successfully marked as read in repository for user %s", notificationID, username)
	return nil
}

// Brisanje notifikacije
func (ns *NotificationService) DeleteNotification(username, notificationID, createdAt string) error {
	logging.Logger.Infof("Service: Attempting to delete notification %s for user %s", notificationID, username)
	if username == "" || notificationID == "" || createdAt == "" {
		err := fmt.Errorf("username, notificationID, and createdAt are required")
		logging.Logger.Warnf("Service: Validation failed for DeleteNotification: %v", err)
		return err
	}
	if err := ns.repo.DeleteNotification(username, notificationID, createdAt); err != nil {
		logging.Logger.Errorf("Service: Failed to delete notification %s in repository for user %s: %v", notificationID, username, err)
		return err
	}
	logging.Logger.Infof("Service: Notification %s successfully deleted in repository for user %s", notificationID, username)
	return nil
}
