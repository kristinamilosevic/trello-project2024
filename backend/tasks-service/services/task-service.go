package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strings"

	"trello-project/microservices/tasks-service/logging"

	"trello-project/microservices/tasks-service/models"

	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TaskService struct {
	tasksCollection      *mongo.Collection
	httpClient           *http.Client
	ProjectsBreaker      *gobreaker.CircuitBreaker
	NotificationsBreaker *gobreaker.CircuitBreaker
	WorkflowBreaker      *gobreaker.CircuitBreaker
}

func NewTaskService(
	tasksCollection *mongo.Collection,
	httpClient *http.Client,
	projectsBreaker *gobreaker.CircuitBreaker,
	notificationsBreaker *gobreaker.CircuitBreaker,
	workflowBreaker *gobreaker.CircuitBreaker,

) *TaskService {
	return &TaskService{
		tasksCollection:      tasksCollection,
		httpClient:           httpClient,
		ProjectsBreaker:      projectsBreaker,
		NotificationsBreaker: notificationsBreaker,
		WorkflowBreaker:      workflowBreaker,
	}
}

func (s *TaskService) GetAvailableMembersForTask(r *http.Request, projectID, taskID string) ([]models.Member, error) {
	projectsServiceURL := os.Getenv("PROJECTS_SERVICE_URL")
	if projectsServiceURL == "" {
		logging.Logger.Warnf("Event ID: CONFIG_ERROR, Description: PROJECTS_SERVICE_URL is not set in .env file for GetAvailableMembersForTask.")
		return nil, fmt.Errorf("projects-service URL is not configured")
	}
	// Napravi URL za HTTP GET zahtev ka projects-service
	url := fmt.Sprintf("%s/api/projects/%s/members/all", projectsServiceURL, projectID)
	logging.Logger.Infof("Event ID: FETCH_PROJECT_MEMBERS, Description: Fetching project members from: %s", url)

	// Dohvati Authorization i Role iz dolaznog HTTP zahteva
	authToken := r.Header.Get("Authorization")
	userRole := r.Header.Get("Role")

	// Napravi HTTP zahtev sa zaglavljima
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logging.Logger.Errorf("Event ID: HTTP_REQUEST_ERROR, Description: Failed to create request to projects-service: %v", err)
		return nil, err
	}

	// Postavi potrebna zaglavlja
	req.Header.Set("Authorization", authToken)
	req.Header.Set("Role", userRole)

	result, err := s.ProjectsBreaker.Execute(func() (interface{}, error) {
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request to projects-service failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("projects-service returned status %d: %s", resp.StatusCode, string(body))
		}

		var rawProjectMembers []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&rawProjectMembers); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return rawProjectMembers, nil
	})
	if err != nil {
		logging.Logger.Warnf("Event ID: CIRCUIT_BREAKER_TRIPPED, Description: Circuit breaker triggered or request to projects-service failed: %v", err)
		logging.Logger.Infof("Event ID: FALLBACK_RESPONSE, Description: Returning empty list of available members as fallback.")
		return []models.Member{}, nil
	}

	// assertuj tip rezultata
	rawProjectMembers := result.([]map[string]interface{})

	// Konvertuj listu članova u models.Member sa ispravnim ID-jem
	var projectMembers []models.Member
	for _, rawMember := range rawProjectMembers {
		idStr, ok := rawMember["_id"].(string)
		if !ok {
			logging.Logger.Warnf("Event ID: INVALID_MEMBER_ID, Description: Member %+v has an invalid _id format.", rawMember)
			continue
		}

		objectID, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			logging.Logger.Warnf("Event ID: ID_CONVERSION_ERROR, Description: Failed to convert member ID %s to ObjectID: %v", idStr, err)
			continue
		}

		member := models.Member{
			ID:       objectID,
			Name:     rawMember["name"].(string),
			LastName: rawMember["lastName"].(string),
			Username: rawMember["username"].(string),
			Role:     rawMember["role"].(string),
		}

		projectMembers = append(projectMembers, member)
	}

	logging.Logger.Infof("Event ID: PROJECT_MEMBERS_FETCHED, Description: Project members fetched and converted for project %s: %+v", projectID, projectMembers)

	// Dohvati podatke o tasku
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		logging.Logger.Errorf("Event ID: INVALID_TASK_ID, Description: Error converting taskID to ObjectID: %s, error: %v", taskID, err)
		return nil, fmt.Errorf("invalid task ID format")
	}

	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_FETCH_ERROR, Description: Failed to fetch task members for taskID: %s, error: %v", taskID, err)
		return nil, fmt.Errorf("failed to fetch task members: %v", err)
	}

	logging.Logger.Infof("Event ID: TASK_MEMBERS_FETCHED, Description: Task members fetched for task %s: %+v", taskID, task.Members)

	// Kreiraj mapu postojećih članova zadatka radi brže provere
	existingTaskMemberIDs := make(map[string]bool)
	for _, taskMember := range task.Members {
		existingTaskMemberIDs[taskMember.ID.Hex()] = true
	}

	// Dodaj u availableMembers samo one koji NISU u tasku
	availableMembers := []models.Member{}
	for _, member := range projectMembers {
		if _, exists := existingTaskMemberIDs[member.ID.Hex()]; !exists {
			logging.Logger.Infof("Event ID: MEMBER_ADDED_TO_AVAILABLE, Description: Adding member %s to available list for task %s", member.Username, taskID)
			availableMembers = append(availableMembers, member)
		} else {
			logging.Logger.Infof("Event ID: MEMBER_SKIPPED, Description: Skipping member %s because they are already in task %s", member.Username, taskID)
		}
	}

	logging.Logger.Infof("Event ID: FINAL_AVAILABLE_MEMBERS, Description: Final available members for task %s: %+v", taskID, availableMembers)

	return availableMembers, nil
}

// Dodaj članove zadatku
func (s *TaskService) AddMembersToTask(taskID string, membersToAdd []models.Member) error {
	// Konvertovanje taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID format")
	}

	// Dohvati zadatak iz baze
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		return fmt.Errorf("task not found: %v", err)
	}

	// Proveri i inicijalizuj polje `members` ako je `nil`
	if task.Members == nil {
		task.Members = []models.Member{}
		_, err := s.tasksCollection.UpdateOne(
			context.Background(),
			bson.M{"_id": taskObjectID},
			bson.M{"$set": bson.M{"members": task.Members}}, // Postavi `members` kao prazan niz
		)
		if err != nil {
			logging.Logger.Errorf("Event ID: MEMBERS_FIELD_INIT_ERROR, Description: Failed to initialize members field for task %s: %v", taskID, err)
			return fmt.Errorf("failed to initialize members field: %v", err)
		}
		logging.Logger.Infof("Event ID: MEMBERS_FIELD_INITIALIZED, Description: Members field initialized as empty array for task %s.", taskID)
	}

	// Filtriraj nove članove koji nisu već dodeljeni
	newMembers := []models.Member{}
	for _, member := range membersToAdd {
		alreadyAssigned := false
		for _, assigned := range task.Members {
			if assigned.ID == member.ID {
				alreadyAssigned = true
				break
			}
		}
		if !alreadyAssigned {
			newMembers = append(newMembers, member)
		}
	}

	if len(newMembers) > 0 {
		// Ažuriraj zadatak sa novim članovima
		update := bson.M{"$addToSet": bson.M{"members": bson.M{"$each": newMembers}}}
		_, err = s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskObjectID}, update)
		if err != nil {
			logging.Logger.Errorf("Event ID: ADD_MEMBERS_TO_TASK_ERROR, Description: Failed to add members to task %s: %v", taskID, err)
			return fmt.Errorf("failed to add members to task: %v", err)
		}
		logging.Logger.Infof("Event ID: MEMBERS_ADDED_TO_TASK, Description: Successfully added %d new members to task %s.", len(newMembers), taskID)

		// Slanje notifikacija za nove članove
		for _, member := range newMembers {
			message := fmt.Sprintf("You have been added to the task: %s!", task.Title)
			go func(member models.Member, message string) {
				_, err := s.NotificationsBreaker.Execute(func() (interface{}, error) {
					return nil, s.sendNotification(member, message)
				})
				if err != nil {
					logging.Logger.Errorf("Event ID: NOTIFICATION_SEND_FAILED, Description: Failed to send notification to member %s for task %s: %v", member.Username, taskID, err)
				}
			}(member, message)
		}
	} else {
		logging.Logger.Infof("Event ID: NO_NEW_MEMBERS, Description: No new members to add to task %s. All provided members are already assigned.", taskID)
	}

	return nil
}

func (s *TaskService) sendNotification(member models.Member, message string) error {
	notification := map[string]string{
		"userId":   member.ID.Hex(),
		"username": member.Username,
		"message":  message, // Dinamična poruka
	}

	notificationData, err := json.Marshal(notification)
	if err != nil {
		logging.Logger.Errorf("Event ID: NOTIFICATION_MARSHAL_ERROR, Description: Error marshaling notification data: %v", err)
		return nil
	}

	notificationURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")
	if notificationURL == "" {
		logging.Logger.Errorf("Event ID: CONFIG_ERROR, Description: Notification service URL is not set in .env")
		return fmt.Errorf("notification service URL is not configured")
	}

	req, err := http.NewRequest("POST", notificationURL, bytes.NewBuffer(notificationData))
	if err != nil {
		logging.Logger.Errorf("Event ID: HTTP_REQUEST_ERROR, Description: Error creating new request for notification: %v", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Role", "manager")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logging.Logger.Errorf("Event ID: HTTP_SEND_ERROR, Description: Error sending HTTP request for notification: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		logging.Logger.Errorf("Event ID: NOTIFICATION_STATUS_ERROR, Description: Failed to create notification, status code: %d", resp.StatusCode)
		return nil
	}

	logging.Logger.Infof("Event ID: NOTIFICATION_SENT, Description: Notification successfully sent for member: %s", member.Username)
	return nil
}

// GetMembersForTask vraća listu članova koji su dodati na određeni task
func (s *TaskService) GetMembersForTask(taskID primitive.ObjectID) ([]models.Member, error) {
	var task models.Task

	// Dohvati zadatak iz baze koristeći ObjectID
	err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_NOT_FOUND, Description: Task not found: %v", err)
		return nil, fmt.Errorf("task not found")
	}

	logging.Logger.Infof("Event ID: MEMBERS_FOR_TASK_FETCHED, Description: Successfully retrieved members for task ID: %s", taskID.Hex())
	// Vrati članove povezane sa zadatkom
	return task.Members, nil
}

func (s *TaskService) CreateTask(projectID string, title, description string, status models.TaskStatus) (*models.Task, error) {
	logging.Logger.Info("⏳ Starting CreateTask...")

	_, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Errorf("❌ Invalid project ID: %v", err)
		return nil, fmt.Errorf("invalid project ID format: %v", err)
	}

	if status == "" {
		logging.Logger.Info("ℹ️ Status not provided, setting to default (pending)")
		status = models.StatusPending
	}

	sanitizedTitle := html.EscapeString(title)
	sanitizedDescription := html.EscapeString(description)

	task := &models.Task{
		ID:          primitive.NewObjectID(),
		ProjectID:   projectID,
		Title:       sanitizedTitle,
		Description: sanitizedDescription,
		Status:      status,
	}

	logging.Logger.Info("📦 Inserting task into MongoDB...")
	result, err := s.tasksCollection.InsertOne(context.Background(), task)
	if err != nil {
		logging.Logger.Errorf("❌ Failed to insert task: %v", err)
		return nil, fmt.Errorf("failed to create task: %v", err)
	}
	task.ID = result.InsertedID.(primitive.ObjectID)
	logging.Logger.Infof("✅ Task inserted with ID: %s", task.ID.Hex())

	// Notify projects-service
	if url := os.Getenv("PROJECTS_SERVICE_URL"); url != "" {
		projectURL := fmt.Sprintf("%s/api/projects/%s/add-task", url, projectID)
		body, _ := json.Marshal(map[string]string{"taskID": task.ID.Hex()})

		req, _ := http.NewRequest("POST", projectURL, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Role", "manager")

		logging.Logger.Infof("📤 Notifying projects-service: %s", projectURL)

		_, err := s.ProjectsBreaker.Execute(func() (interface{}, error) {
			resp, err := s.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("projects-service error: %s", string(body))
			}
			return nil, nil
		})

		if err != nil {
			logging.Logger.Warnf("🔁 Fallback: Failed to notify projects-service for task %s: %v", task.ID.Hex(), err)
		} else {
			logging.Logger.Infof("✅ projects-service notified about task %s", task.ID.Hex())
		}
	} else {
		logging.Logger.Warn("⚠️ PROJECTS_SERVICE_URL is not set")
	}

	// Notify workflow-service
	if url := os.Getenv("WORKFLOW_SERVICE_URL"); url != "" {
		workflowURL := fmt.Sprintf("%s/api/workflow/task-node", strings.TrimRight(url, "/"))
		payload := map[string]any{
			"id":          task.ID.Hex(),
			"projectId":   task.ProjectID,
			"name":        task.Title,
			"description": task.Description,
			"blocked":     false,
		}
		body, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", workflowURL, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		logging.Logger.Infof("📤 Notifying workflow-service: %s", workflowURL)

		_, err := s.WorkflowBreaker.Execute(func() (interface{}, error) {
			resp, err := s.httpClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				body, _ := io.ReadAll(resp.Body)
				return nil, fmt.Errorf("workflow-service error: %s", string(body))
			}
			return nil, nil
		})

		if err != nil {
			logging.Logger.Warnf("🔁 Fallback: Failed to notify workflow-service for task %s: %v", task.ID.Hex(), err)
			// Optionally: queue for retry, etc.
		} else {
			logging.Logger.Infof("✅ workflow-service notified about task %s", task.ID.Hex())
		}
	} else {
		logging.Logger.Warn("⚠️ WORKFLOW_SERVICE_URL is not set")
	}

	logging.Logger.Info("✅ Task creation process completed.")
	return task, nil
}

func (s *TaskService) GetAllTasks() ([]*models.Task, error) {
	var tasks []*models.Task
	cursor, err := s.tasksCollection.Find(context.Background(), bson.M{})
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_RETRIEVAL_FAILED, Description: Failed to retrieve tasks: %v", err)
		return nil, fmt.Errorf("failed to retrieve tasks: %v", err)
	}
	defer cursor.Close(context.Background())

	// Iteracija kroz sve zadatke
	for cursor.Next(context.Background()) {
		var task models.Task
		if err := cursor.Decode(&task); err != nil {
			logging.Logger.Errorf("Event ID: TASK_DECODE_FAILED, Description: Failed to decode task: %v", err)
			return nil, fmt.Errorf("failed to decode task: %v", err)
		}
		tasks = append(tasks, &task)
	}

	if err := cursor.Err(); err != nil {
		logging.Logger.Errorf("Event ID: CURSOR_ERROR, Description: Cursor error during task retrieval: %v", err)
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	logging.Logger.Infof("Event ID: ALL_TASKS_RETRIEVED, Description: Successfully retrieved %d tasks.", len(tasks))
	return tasks, nil
}

func (s *TaskService) RemoveMemberFromTask(taskID string, memberID primitive.ObjectID) error {
	// Konvertovanje taskID u ObjectID
	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		logging.Logger.Errorf("Event ID: INVALID_TASK_ID_FORMAT, Description: Invalid task ID format: %v", err)
		return fmt.Errorf("invalid task ID format")
	}

	// Dohvatanje zadatka iz baze
	var task models.Task
	err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskObjectID}).Decode(&task)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_NOT_FOUND, Description: Task not found with ID %s: %v", taskID, err)
		return fmt.Errorf("task not found: %v", err)
	}

	// ❗️Provera da li je task završen
	if task.Status == "Completed" {
		logging.Logger.Warnf("Event ID: REMOVE_FROM_COMPLETED_TASK_ATTEMPT, Description: Attempted to remove member from a completed task %s.", taskID)
		return fmt.Errorf("cannot remove member from a completed task")
	}

	// Provera da li je član deo zadatka
	memberFound := false
	var removedMember models.Member
	for i, member := range task.Members {
		if member.ID == memberID {
			removedMember = member
			task.Members = append(task.Members[:i], task.Members[i+1:]...)
			memberFound = true
			break
		}
	}

	if !memberFound {
		logging.Logger.Warnf("Event ID: MEMBER_NOT_FOUND_IN_TASK, Description: Member %s not found in task %s.", memberID.Hex(), taskID)
		return fmt.Errorf("member not found in the task")
	}

	// Ažuriranje zadatka u bazi
	_, err = s.tasksCollection.UpdateOne(
		context.Background(),
		bson.M{"_id": taskObjectID},
		bson.M{"$set": bson.M{"members": task.Members}},
	)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASK_UPDATE_FAILED, Description: Failed to update task %s after member removal: %v", taskID, err)
		return fmt.Errorf("failed to update task: %v", err)
	}
	logging.Logger.Infof("Event ID: MEMBER_REMOVED_FROM_TASK, Description: Successfully removed member %s from task %s.", memberID.Hex(), taskID)

	// Asinhrono slanje notifikacije preko Circuit Breaker-a
	message := fmt.Sprintf("You have been removed from the task: %s", task.Title)
	go func(member models.Member, message string) {
		_, err := s.NotificationsBreaker.Execute(func() (interface{}, error) {
			return nil, s.sendNotification(member, message)
		})
		if err != nil {
			logging.Logger.Errorf("Event ID: NOTIFICATION_SEND_FAILED, Description: Failed to send removal notification to member %s: %v", member.Username, err)
		}
	}(removedMember, message)

	return nil
}

func (s *TaskService) GetTaskByID(taskID primitive.ObjectID) (*models.Task, error) {
	var task models.Task
	err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task)
	if err != nil {
		logging.Logger.Warnf("Event ID: TASK_NOT_FOUND, Description: Task with ID %s not found: %v", taskID.Hex(), err)
		return nil, err
	}
	logging.Logger.Infof("Event ID: TASK_FETCHED_BY_ID, Description: Successfully retrieved task with ID: %s", taskID.Hex())
	return &task, nil
}

func (s *TaskService) ChangeTaskStatus(taskID primitive.ObjectID, status models.TaskStatus, username string) (*models.Task, error) {
	var task models.Task
	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("task not found: %v", err)
	}

	logging.Logger.Infof("Task '%s' current status: %s", task.Title, task.Status)
	logging.Logger.Infof("Attempting to change status to: %s", status)

	isAuthorized := false
	for _, member := range append(task.Members, task.Assignees...) {
		if member.Username == username {
			isAuthorized = true
			break
		}
	}
	if !isAuthorized {
		return nil, fmt.Errorf("user '%s' is not authorized to change the status of this task", username)
	}

	var dependencyIDs []string
	var err error
	if status == models.StatusInProgress || status == models.StatusCompleted {
		dependencyIDs, err = s.getDependenciesFromWorkflow(task.ID.Hex())
		if err != nil {
			return nil, fmt.Errorf("failed to get dependencies from workflow service: %v", err)
		}

		for _, depIDStr := range dependencyIDs {
			depID, err := primitive.ObjectIDFromHex(depIDStr)
			if err != nil {
				continue
			}

			var depTask models.Task
			err = s.tasksCollection.FindOne(context.Background(), bson.M{"_id": depID}).Decode(&depTask)
			if err != nil {
				return nil, fmt.Errorf("dependent task not found: %v", err)
			}

			if depTask.Status != models.StatusInProgress && depTask.Status != models.StatusCompleted {
				return nil, fmt.Errorf("cannot change status: dependent task '%s' is neither in progress nor completed", depTask.Title)
			}
		}
	} else {
		dependencyIDs, _ = s.getDependenciesFromWorkflow(task.ID.Hex())
	}

	update := bson.M{"$set": bson.M{"status": status}}
	_, err = s.tasksCollection.UpdateOne(context.Background(), bson.M{"_id": taskID}, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update task status: %v", err)
	}

	if err := s.tasksCollection.FindOne(context.Background(), bson.M{"_id": taskID}).Decode(&task); err != nil {
		return nil, fmt.Errorf("failed to fetch updated task: %v", err)
	}

	logging.Logger.Infof("✅ Successfully updated task '%s' to status: %s", task.Title, task.Status)

	isBlocked := false
	if len(dependencyIDs) > 0 {
		isBlocked = true
	} else if status == models.StatusInProgress {
		isBlocked = true
	}

	err = s.updateBlockedInWorkflow(task.ID.Hex(), isBlocked)
	if err != nil {
		logging.Logger.Warnf("⚠️ Failed to refresh blocked status in workflow-service: %v", err)
	}

	message := fmt.Sprintf("The status of task '%s' has been changed to: %s", task.Title, status)
	for _, member := range append(task.Members, task.Assignees...) {
		err := s.sendNotification(member, message)
		if err != nil {
			logging.Logger.Warnf("⚠️ Failed to notify user %s: %v", member.Username, err)
		}
	}

	return &task, nil
}

func (s *TaskService) DeleteTasksByProject(projectID string) error {
	filter := bson.M{"projectId": projectID}

	result, err := s.tasksCollection.DeleteMany(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASKS_DELETE_FAILED, Description: Failed to delete tasks for project ID %s: %v", projectID, err)
		return fmt.Errorf("failed to delete tasks: %v", err)
	}

	logging.Logger.Infof("Event ID: TASKS_DELETED_BY_PROJECT, Description: Successfully deleted %d tasks for project ID %s", result.DeletedCount, projectID)
	return nil
}

func (s *TaskService) HasActiveTasks(ctx context.Context, projectID, memberID string) (bool, error) {
	memberObjectID, err := primitive.ObjectIDFromHex(memberID)
	if err != nil {
		logging.Logger.Errorf("Event ID: INVALID_MEMBER_ID_FORMAT, Description: Invalid memberID: %s. Error: %v", memberID, err)
		return false, err
	}

	filter := bson.M{
		"projectId":   projectID,
		"members._id": memberObjectID,
		"status":      "In progress",
	}

	count, err := s.tasksCollection.CountDocuments(ctx, filter)
	if err != nil {
		logging.Logger.Errorf("Event ID: COUNT_ACTIVE_TASKS_FAILED, Description: Failed to count active tasks for project %s and member %s: %v", projectID, memberID, err)
		return false, err
	}

	logging.Logger.Infof("Event ID: ACTIVE_TASKS_COUNT, Description: Found %d active tasks for project %s and member %s", count, projectID, memberID)

	return count > 0, nil
}

func (s *TaskService) GetTasksByProjectID(projectID string) ([]models.Task, error) {
	filter := bson.M{"projectId": projectID}
	cursor, err := s.tasksCollection.Find(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Event ID: TASKS_BY_PROJECT_FETCH_FAILED, Description: Failed to find tasks for project %s: %v", projectID, err)
		return nil, fmt.Errorf("failed to find tasks: %w", err)
	}
	defer cursor.Close(context.Background())

	var tasks []models.Task
	if err := cursor.All(context.Background(), &tasks); err != nil {
		logging.Logger.Errorf("Event ID: TASKS_DECODE_FAILED, Description: Failed to decode tasks for project %s: %v", projectID, err)
		return nil, fmt.Errorf("failed to decode tasks: %w", err)
	}
	logging.Logger.Infof("Event ID: TASKS_BY_PROJECT_RETRIEVED, Description: Successfully retrieved %d tasks for project ID %s.", len(tasks), projectID)
	return tasks, nil
}

func HasUnfinishedTasks(tasks []models.Task) bool {
	for _, task := range tasks {
		if task.Status != models.StatusCompleted {
			logging.Logger.Debugf("Event ID: UNFINISHED_TASK_FOUND, Description: Unfinished task '%s' found (Status: %s).", task.Title, task.Status)
			return true
		}
	}
	logging.Logger.Debugf("Event ID: NO_UNFINISHED_TASKS, Description: No unfinished tasks found in the provided list.")
	return false
}

func (s *TaskService) RemoveUserFromAllTasksByUsername(username string) error {
	// Uklanjanje iz members
	filterMembers := bson.M{
		"members.username": username,
	}
	updateMembers := bson.M{
		"$pull": bson.M{
			"members": bson.M{"username": username},
		},
	}
	resultMembers, err := s.tasksCollection.UpdateMany(context.Background(), filterMembers, updateMembers)
	if err != nil {
		logging.Logger.Errorf("Event ID: REMOVE_USER_FROM_MEMBERS_FAILED, Description: Failed to remove user '%s' from members: %v", username, err)
		return fmt.Errorf("failed to remove user from members: %v", err)
	}

	// Uklanjanje iz assignees
	filterAssignees := bson.M{
		"$and": []bson.M{
			{"assignees": bson.M{"$type": "array"}},
			{"assignees": username},
		},
	}
	updateAssignees := bson.M{
		"$pull": bson.M{
			"assignees": username,
		},
	}
	resultAssignees, err := s.tasksCollection.UpdateMany(context.Background(), filterAssignees, updateAssignees)
	if err != nil {
		logging.Logger.Errorf("Event ID: REMOVE_USER_FROM_ASSIGNEES_FAILED, Description: Failed to remove user '%s' from assignees: %v", username, err)
		return fmt.Errorf("failed to remove user from assignees: %v", err)
	}

	totalModified := resultMembers.ModifiedCount + resultAssignees.ModifiedCount
	logging.Logger.Infof("Event ID: REMOVE_USER_FROM_ALL_TASKS_SUCCESS, Description: Successfully removed user '%s' from %d tasks.", username, totalModified)

	return nil
}

func (s *TaskService) getDependenciesFromWorkflow(taskID string) ([]string, error) {
	result, err := s.WorkflowBreaker.Execute(func() (interface{}, error) {
		baseURL := os.Getenv("WORKFLOW_SERVICE_URL")
		if baseURL == "" {
			return nil, fmt.Errorf("WORKFLOW_SERVICE_URL not set in environment")
		}

		url := fmt.Sprintf("%s/api/workflow/dependencies/%s", baseURL, taskID)

		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("failed to contact workflow-service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("workflow-service returned status: %d", resp.StatusCode)
		}

		var dependencies []struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&dependencies); err != nil {
			return nil, fmt.Errorf("failed to decode response: %v", err)
		}

		var ids []string
		for _, dep := range dependencies {
			ids = append(ids, dep.ID)
		}
		return ids, nil
	})

	if err != nil {
		// Fallback logika
		fmt.Printf("Fallback triggered: %v\n", err)
		return []string{}, nil // ili neka default vrednost
	}

	return result.([]string), nil
}

func (s *TaskService) updateBlockedInWorkflow(taskID string, blocked bool) error {
	workflowURL := os.Getenv("WORKFLOW_SERVICE_URL")
	if workflowURL == "" {
		return fmt.Errorf("WORKFLOW_SERVICE_URL not set")
	}

	url := fmt.Sprintf("%s/api/workflow/task-node/%s/blocked", workflowURL, taskID)
	payload := map[string]bool{
		"blocked": blocked,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Wrap HTTP logic in circuit breaker execution
	_, err = s.WorkflowBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send HTTP request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("workflow-service returned status code %d", resp.StatusCode)
		}

		return nil, nil
	})

	// Fallback logika kada je circuit breaker otvoren
	if errors.Is(err, gobreaker.ErrOpenState) {
		logging.Logger.Warnf("[updateBlockedInWorkflow] Circuit breaker OPEN. Skipping update for taskID %s, blocked=%v", taskID, blocked)
		return nil // fallback: ne pravimo problem ako ne možemo da ažuriramo
	}

	if err != nil {
		logging.Logger.Errorf("[updateBlockedInWorkflow] Error updating workflow service for taskID %s: %v", taskID, err)
		return err
	}

	return nil
}
