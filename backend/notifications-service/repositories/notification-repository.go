package repositories

import (
	"notifications-service/models"
	"os"
	"time"

	"github.com/gocql/gocql"
	"github.com/sirupsen/logrus"
)

type NotificationRepo struct {
	session *gocql.Session
	logger  *logrus.Logger
}

// Konstruktor za povezivanje na Cassandra bazu
func NewNotificationRepo(logger *logrus.Logger) (*NotificationRepo, error) {
	db := os.Getenv("CASS_DB") // Preuzimanje konfiguracije iz okruženja
	if db == "" {
		db = "127.0.0.1" // Podrazumevana vrednost za lokalnu Cassandru
	}

	cluster := gocql.NewCluster(db)
	cluster.Keyspace = "system"
	session, err := cluster.CreateSession()
	if err != nil {
		logger.Errorf("Failed to create Cassandra session: %v", err)
		return nil, err
	}

	// Kreiranje keyspace-a ako ne postoji
	err = session.Query(
		`CREATE KEYSPACE IF NOT EXISTS notifications 
         WITH replication = {
             'class': 'SimpleStrategy',
             'replication_factor': 1
         }`).Exec()
	if err != nil {
		logger.Errorf("Failed to create keyspace: %v", err)
		return nil, err
	}
	session.Close()

	// Povezivanje na keyspace notifications
	cluster.Keyspace = "notifications"
	cluster.Consistency = gocql.One
	session, err = cluster.CreateSession()
	if err != nil {
		logger.Errorf("Failed to connect to notifications keyspace: %v", err)
		return nil, err
	}

	logger.Info("Connected to Cassandra notifications keyspace.")
	return &NotificationRepo{
		session: session,
		logger:  logger,
	}, nil
}

// Funkcija za zatvaranje sesije
func (nr *NotificationRepo) CloseSession() {
	nr.session.Close()
	nr.logger.Infof("Cassandra session closed.")
}

// Kreiranje tabele za notifikacije
func (nr *NotificationRepo) CreateTable() {
	err := nr.session.Query(
		`CREATE TABLE IF NOT EXISTS notifications (
			id UUID,
			username TEXT,
			user_id TEXT,
			message TEXT,
			created_at TIMESTAMP,
			is_read BOOLEAN,
			PRIMARY KEY ((username), created_at, id)
		) WITH CLUSTERING ORDER BY (created_at DESC, id ASC)`).Exec()
	if err != nil {
		nr.logger.Errorf("Failed to create notifications table: %v", err)
	} else {
		nr.logger.Info("Notifications table created successfully!")
	}
}

func (nr *NotificationRepo) CreateNotification(notification *models.Notification) error {
	if notification.ID == "" {
		notification.ID = gocql.TimeUUID().String()
	}

	err := nr.session.Query(
		`INSERT INTO notifications (id, username, user_id, message, created_at, is_read)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		notification.ID, notification.Username, notification.UserID, notification.Message, notification.CreatedAt, notification.IsRead,
	).Exec()
	if err != nil {
		nr.logger.Errorf("Error creating notification: %v", err)
		return err
	}

	nr.logger.Infof("Notification successfully created for user %s (ID: %s)!", notification.Username, notification.ID)
	return nil
}

func (nr *NotificationRepo) GetNotificationsByUsername(username string) ([]models.Notification, error) {
	query := `SELECT id, user_id, username, message, created_at, is_read 
			  FROM notifications WHERE username = ?`

	// Iteriraj kroz rezultate
	iter := nr.session.Query(query, username).Iter()
	var notifications []models.Notification
	var notification models.Notification

	nr.logger.Infof("Fetching notifications for username: %s", username)

	for iter.Scan(&notification.ID, &notification.UserID, &notification.Username,
		&notification.Message, &notification.CreatedAt, &notification.IsRead) {
		notifications = append(notifications, notification)
	}

	if err := iter.Close(); err != nil {
		nr.logger.Errorf("Error fetching notifications by username %s: %v", username, err)
		return nil, err
	}
	nr.logger.Infof("Successfully fetched %d notifications for username: %s", len(notifications), username)
	return notifications, nil
}

func (nr *NotificationRepo) MarkNotificationAsRead(username, notificationID, createdAt string) error {
	// Parsiranje UUID-a za `id`
	uuid, err := gocql.ParseUUID(notificationID)
	if err != nil {
		nr.logger.Errorf("Invalid UUID format for notification ID %s: %v", notificationID, err)
		return err
	}

	// Parsiranje vremena za `created_at`
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		nr.logger.Errorf("Invalid created_at format %s: %v", createdAt, err)
		return err
	}
	nr.logger.Infof("Attempting to mark notification ID %s as read for user %s", notificationID, username)

	// Ažuriranje u Cassandri
	query := `UPDATE notifications SET is_read = true WHERE username = ? AND id = ? AND created_at = ?`
	err = nr.session.Query(query, username, uuid, parsedCreatedAt).Exec()
	if err != nil {
		nr.logger.Errorf("Error updating notification ID %s to read status: %v", notificationID, err)
		return err
	}

	nr.logger.Infof("Notification ID %s successfully marked as read for user %s!", notificationID, username)
	return nil
}

func (nr *NotificationRepo) DeleteNotification(username, notificationID, createdAt string) error {
	// Provera ulaznih podataka
	nr.logger.Debugf("Received for deletion: username=%s, id=%s, created_at=%s", username, notificationID, createdAt)
	uuid, err := gocql.ParseUUID(notificationID)
	if err != nil {
		nr.logger.Errorf("Invalid UUID format for deletion, ID %s: %v", notificationID, err)
		return err
	}
	nr.logger.Debugf("Parsed UUID for deletion: %s", uuid)
	// Konverzija vremena
	parsedCreatedAt, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		nr.logger.Errorf("Invalid created_at format for deletion %s: %v", createdAt, err)
		return err
	}
	nr.logger.Debugf("Parsed CreatedAt for deletion: %s", parsedCreatedAt)

	// Izvršavanje upita
	query := `DELETE FROM notifications WHERE username = ? AND id = ? AND created_at = ?`
	err = nr.session.Query(query, username, uuid, parsedCreatedAt).Exec()
	if err != nil {
		nr.logger.Errorf("Error deleting notification ID %s: %v", notificationID, err)
		return err
	}

	nr.logger.Infof("Notification ID %s successfully deleted!", notificationID)
	return nil
}
