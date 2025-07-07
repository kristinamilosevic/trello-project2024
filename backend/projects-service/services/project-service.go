package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"trello-project/microservices/projects-service/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	ProjectsCollection *mongo.Collection
	TasksCollection    *mongo.Collection
	UsersCollection    *mongo.Collection
	HTTPClient         *http.Client
}

// NewProjectService initializes a new ProjectService with the necessary MongoDB collections.
func NewProjectService(projectsCollection, usersCollection, tasksCollection *mongo.Collection, httpClient *http.Client) *ProjectService {
	return &ProjectService{
		ProjectsCollection: projectsCollection,
		UsersCollection:    usersCollection,
		TasksCollection:    tasksCollection,
		HTTPClient:         httpClient,
	}
}

// CreateProject creates a new project with the specified parameters.
func (s *ProjectService) CreateProject(name string, description string, expectedEndDate time.Time, minMembers, maxMembers int, managerID primitive.ObjectID) (*models.Project, error) {
	var existingProject models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"name": name}).Decode(&existingProject)
	if err == nil {
		return nil, errors.New("project with the same name already exists")
	}

	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("database error: %v", err)
	}
	if minMembers < 1 || maxMembers < minMembers {
		return nil, fmt.Errorf("invalid member constraints: minMembers=%d, maxMembers=%d", minMembers, maxMembers)
	}
	if expectedEndDate.Before(time.Now()) {
		return nil, fmt.Errorf("expected end date must be in the future")
	}

	sanitizedName := html.EscapeString(name)
	sanitizedDescription := html.EscapeString(description)

	project := &models.Project{
		ID:              primitive.NewObjectID(),
		Name:            sanitizedName,
		Description:     sanitizedDescription,
		ExpectedEndDate: expectedEndDate,
		MinMembers:      minMembers,
		MaxMembers:      maxMembers,
		ManagerID:       managerID,
		Members:         []models.Member{},
		Tasks:           []primitive.ObjectID{},
	}

	result, err := s.ProjectsCollection.InsertOne(context.Background(), project)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %v", err)
	}

	project.ID = result.InsertedID.(primitive.ObjectID)
	return project, nil
}

// AddMembersToProject adds multiple members to a project.
func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, usernames []string) error {
	var project models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": projectID}).Decode(&project)
	if err != nil {
		log.Printf("Error finding project: %v\n", err)
		return fmt.Errorf("project not found: %v", err)
	}

	// Provera maksimalnog broja članova
	if len(project.Members)+len(usernames) > project.MaxMembers {
		log.Println("Maximum number of members reached for the project")
		return fmt.Errorf("maximum number of members reached for the project")
	}

	// Filtriranje članova koji su već u projektu
	existingMemberUsernames := make(map[string]bool)
	for _, member := range project.Members {
		existingMemberUsernames[member.Username] = true
	}

	var newUsernames []string
	for _, username := range usernames {
		if !existingMemberUsernames[username] {
			newUsernames = append(newUsernames, username)
		} else {
			log.Printf("Member %s is already in the project, skipping.\n", username)
		}
	}

	if len(newUsernames) == 0 {
		log.Println("No new members to add.")
		return fmt.Errorf("all provided members are already part of the project")
	}

	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	var members []models.Member

	for _, username := range newUsernames {
		url := fmt.Sprintf("%s/api/users/member/%s", usersServiceURL, username)
		log.Printf("Fetching user data from: %s\n", url)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("Error creating request for user %s: %v\n", username, err)
			return err
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			log.Printf("Failed to fetch member %s: %v\n", username, err)
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to fetch member %s, status code: %d\n", username, resp.StatusCode)
			return fmt.Errorf("failed to fetch member %s", username)
		}

		var user models.Member
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			log.Printf("Failed to decode member %s: %v\n", username, err)
			return err
		}

		members = append(members, user)
	}

	// Ažuriranje baze sa novim članovima
	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err = s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Error updating project members: %v\n", err)
		return err
	}

	log.Println("Members successfully added to the project.")
	return nil
}

func (s *ProjectService) sendNotification(member models.Member, message string) error {
	notification := map[string]string{
		"userId":   member.ID.Hex(),
		"username": member.Username,
		"message":  message, // Dinamična poruka
	}

	notificationData, err := json.Marshal(notification)
	if err != nil {
		fmt.Printf("Error marshaling notification data: %v\n", err)
		return nil
	}

	// Učitaj URL iz .env fajla
	notificationURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")
	if notificationURL == "" {
		fmt.Println("Notification service URL is not set in .env")
		return fmt.Errorf("notification service URL is not configured")
	}

	req, err := http.NewRequest("POST", notificationURL, bytes.NewBuffer(notificationData))
	if err != nil {
		fmt.Printf("Error creating new request: %v\n", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Role", "manager")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("Error sending HTTP request: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		fmt.Printf("Failed to create notification, status code: %d\n", resp.StatusCode)
		return nil
	}

	fmt.Printf("Notification successfully sent for member: %s\n", member.Username)
	return nil
}

// GetProjectMembers retrieves members of a specific project.
func (s *ProjectService) GetProjectMembers(ctx context.Context, projectID string) ([]bson.M, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		fmt.Println("Invalid project ID format:", err)
		return nil, err
	}

	var project struct {
		Members []bson.M `bson:"members"`
	}

	err = s.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		fmt.Println("Error fetching project members from database:", err)
		return nil, err
	}

	return project.Members, nil
}

func (s *ProjectService) GetAllUsers() ([]models.Member, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	url := fmt.Sprintf("%s/api/users/members", usersServiceURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch users from users-service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("users-service returned status: %d", resp.StatusCode)
	}

	var users []models.Member
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, fmt.Errorf("failed to decode response from users-service: %v", err)
	}

	return users, nil
}

// RemoveMemberFromProject removes a member from a project if they are not assigned to an in-progress task.
func (s *ProjectService) RemoveMemberFromProject(ctx context.Context, projectID, memberID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Println("Invalid project ID format")
		return fmt.Errorf("invalid project ID format")
	}

	memberObjectID, err := primitive.ObjectIDFromHex(memberID)
	if err != nil {
		log.Println("Invalid member ID format")
		return fmt.Errorf("invalid member ID format")
	}

	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if taskServiceURL == "" {
		log.Println("TASKS_SERVICE_URL is not set")
		return fmt.Errorf("task service URL is not configured")
	}

	checkURL := fmt.Sprintf("%s/api/tasks/has-active?projectId=%s&memberId=%s", taskServiceURL, projectID, memberID)

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		log.Printf("Failed to create HTTP request: %v\n", err)
		return fmt.Errorf("failed to create HTTP request")
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		log.Printf("Failed to reach task service: %v\n", err)
		return fmt.Errorf("failed to connect to task service")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Task service returned status %d: %s\n", resp.StatusCode, string(body))
		return fmt.Errorf("task service returned error %d", resp.StatusCode)
	}

	var response struct {
		HasActiveTasks bool `json:"hasActiveTasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		log.Printf("Failed to decode task service response: %v\n", err)
		return fmt.Errorf("failed to decode task service response")
	}

	if response.HasActiveTasks {
		log.Println("Cannot remove member assigned to an active task")
		return fmt.Errorf("cannot remove member assigned to an active task")
	}

	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"_id": memberObjectID}}}

	result, err := s.ProjectsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Println("Failed to remove member from project")
		return fmt.Errorf("failed to remove member from project")
	}

	if result.ModifiedCount == 0 {
		log.Println("Member not found in project or already removed")
		return fmt.Errorf("member not found in project or already removed")
	}

	log.Println("Member successfully removed from project.")
	return nil
}

// GetAllProjects - preuzima sve projekte iz kolekcije
func (s *ProjectService) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	cursor, err := s.ProjectsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		return nil, fmt.Errorf("unsuccessful procurement of projects: %v", err)
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &projects); err != nil {
		return nil, fmt.Errorf("unsuccessful decoding of projects: %v", err)
	}

	return projects, nil
}

func (s *ProjectService) GetProjectByID(projectID string) (*models.Project, error) {
	objectId, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		fmt.Println("Invalid project ID format:", projectID)
		return nil, fmt.Errorf("invalid project ID format")
	}

	var project models.Project
	err = s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("error fetching project: %v", err)
	}
	return &project, nil
}

func (s *ProjectService) GetTasksForProject(projectID string, role string, authToken string) ([]map[string]interface{}, error) {
	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if tasksServiceURL == "" {
		return nil, fmt.Errorf("TASKS_SERVICE_URL not set")
	}

	url := fmt.Sprintf("%s/api/tasks/project/%s", tasksServiceURL, projectID)
	fmt.Printf("Fetching tasks from: %s\n", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Role", role)
	req.Header.Set("Authorization", authToken)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch tasks for project %s: %v\n", projectID, err)
		return nil, fmt.Errorf("failed to fetch tasks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to fetch tasks for project %s, status code: %d\n", projectID, resp.StatusCode)
		return nil, fmt.Errorf("failed to fetch tasks, status code: %d", resp.StatusCode)
	}

	var tasks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		log.Printf("Failed to decode tasks response: %v\n", err)
		return nil, fmt.Errorf("failed to decode tasks: %v", err)
	}

	return tasks, nil
}
func (s *ProjectService) getUserIDByUsername(username string) (primitive.ObjectID, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		return primitive.NilObjectID, fmt.Errorf("USERS_SERVICE_URL not set")
	}

	url := fmt.Sprintf("%s/api/users/id/%s", usersServiceURL, username)
	resp, err := http.Get(url)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to contact users-service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return primitive.NilObjectID, fmt.Errorf("users-service returned status: %v", resp.Status)
	}

	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to decode user ID response: %v", err)
	}

	userID, err := primitive.ObjectIDFromHex(data.ID)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("invalid user ID format: %v", err)
	}

	return userID, nil
}
func (s *ProjectService) getUserRoleByUsername(username string) (string, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		return "", fmt.Errorf("USERS_SERVICE_URL not set")
	}

	url := fmt.Sprintf("%s/api/users/role/%s", usersServiceURL, username)
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to contact users-service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("users-service returned status: %v", resp.Status)
	}

	var data struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", fmt.Errorf("failed to decode role response: %v", err)
	}

	return data.Role, nil
}

func (s *ProjectService) GetProjectsByUsername(username string) ([]models.Project, error) {
	var projects []models.Project

	// Dobavi userID iz username
	userID, err := s.getUserIDByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user ID: %v", err)
	}

	// Dobavi role korisnika
	role, err := s.getUserRoleByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %v", err)
	}

	// Formiraj filter na osnovu role
	var filter bson.M
	if role == "manager" {
		filter = bson.M{"manager_id": userID}
	} else {
		filter = bson.M{"members.username": username}
	}

	log.Printf("Executing MongoDB query with filter: %v", filter)

	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		log.Printf("Error fetching projects from MongoDB: %v", err)
		return nil, fmt.Errorf("error fetching projects: %v", err)
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			log.Printf("Error decoding project document: %v", err)
			return nil, fmt.Errorf("error decoding project: %v", err)
		}
		projects = append(projects, project)
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return nil, fmt.Errorf("cursor error: %v", err)
	}

	log.Printf("Found %d projects for username %s", len(projects), username)
	return projects, nil
}

func (s *ProjectService) DeleteProjectAndTasks(ctx context.Context, projectID string, r *http.Request) error {
	// Konverzija projectID u ObjectID
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Printf("Invalid project ID format: %v", projectID)
		return fmt.Errorf("invalid project ID format")
	}

	// Provera postojanja projekta
	filter := bson.M{"_id": projectObjectID}
	var project bson.M
	err = s.ProjectsCollection.FindOne(ctx, filter).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Printf("Project not found: %v", projectID)
			return fmt.Errorf("project not found")
		}
		log.Printf("Failed to fetch project: %v", err)
		return fmt.Errorf("failed to fetch project: %v", err)
	}

	// Priprema zahteva za tasks-service
	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/tasks/project/%s", taskServiceURL, projectID), nil)
	if err != nil {
		log.Printf("Failed to create request to tasks-service: %v", err)
		return fmt.Errorf("failed to create request to task service: %v", err)
	}

	// Prosleđivanje zaglavlja
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Role", r.Header.Get("Role"))

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		log.Printf("Failed to send request to tasks-service: %v", err)
		return fmt.Errorf("failed to send request to task service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Tasks-service returned non-OK status: %v", resp.Status)
		return fmt.Errorf("task service returned an error: %v", resp.Status)
	}

	// Brisanje projekta iz baze
	_, err = s.ProjectsCollection.DeleteOne(ctx, filter)
	if err != nil {
		log.Printf("Failed to delete project: %v", err)
		return fmt.Errorf("failed to delete project: %v", err)
	}

	log.Printf("Successfully deleted project and related tasks for ID: %s", projectID)
	return nil
}

func (s *ProjectService) GetAllMembers() ([]models.Member, error) {
	// Učitaj URL `users-service` iz .env fajla
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		log.Println("USERS_SERVICE_URL is not set in .env file")
		return nil, fmt.Errorf("users-service URL is not configured")
	}

	// Priprema HTTP GET zahteva
	req, err := http.NewRequest("GET", usersServiceURL+"/api/users/members", nil)
	if err != nil {
		log.Printf("Failed to create request for users-service: %v", err)
		return nil, err
	}

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		log.Printf("Failed to fetch members from users-service: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("users-service returned non-200 status: %d", resp.StatusCode)
		return nil, fmt.Errorf("failed to get members, status: %d", resp.StatusCode)
	}

	// Dekodiraj odgovor
	var members []models.Member
	if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
		log.Printf("Failed to decode members response: %v", err)
		return nil, err
	}

	return members, nil
}

func (s *ProjectService) AddTaskToProject(projectID string, taskID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		return fmt.Errorf("invalid project ID format: %v", err)
	}

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return fmt.Errorf("invalid task ID format: %v", err)
	}

	log.Printf("Received request to add task %s to project %s", taskID, projectID)

	// Ažuriranje projekta dodavanjem ID-ja zadatka
	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$push": bson.M{"taskIDs": taskObjectID}}

	log.Printf("MongoDB filter: %+v", filter)
	log.Printf("MongoDB update: %+v", update)

	result, err := s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		log.Printf("Failed to update project with task ID: %v", err)
		return fmt.Errorf("failed to update project with task ID: %v", err)
	}

	if result.ModifiedCount == 0 {
		log.Printf("No project was updated. Possible that project ID %s does not exist.", projectID)
		return fmt.Errorf("no project found with ID %s", projectID)
	}

	log.Printf("Task %s successfully added to project %s", taskID, projectID)
	return nil
}

func (s *ProjectService) RemoveUserFromProjects(userID string, role string, authToken string) error {
	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if tasksServiceURL == "" {
		return fmt.Errorf("TASKS_SERVICE_URL not set")
	}

	if role == "manager" {
		projectFilter := bson.M{"manager_id": userID}
		cursor, err := s.ProjectsCollection.Find(context.Background(), projectFilter)
		if err != nil {
			log.Printf("Error fetching projects for manager %s: %v\n", userID, err)
			return fmt.Errorf("failed to fetch projects")
		}
		defer cursor.Close(context.Background())

		for cursor.Next(context.Background()) {
			var project models.Project
			if err := cursor.Decode(&project); err != nil {
				log.Printf("Error decoding project: %v\n", err)
				continue
			}

			// Pripremi zahtev sa Authorization header-om
			url := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", tasksServiceURL, project.ID.Hex())
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				log.Printf("Failed to create request: %v", err)
				continue
			}
			req.Header.Set("Authorization", "Bearer "+authToken)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Failed to contact task service: %v\n", err)
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				log.Printf("Task service returned non-OK status for project %s: %s\n", project.ID.Hex(), resp.Status)
				continue
			}

			var result struct {
				HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				log.Printf("Failed to decode task service response: %v\n", err)
				continue
			}

			if result.HasUnfinishedTasks {
				log.Printf("Cannot remove manager %s: unfinished tasks in project %s\n", userID, project.ID.Hex())
				return fmt.Errorf("manager cannot be removed from project %s due to unfinished tasks", project.ID.Hex())
			}
		}

		update := bson.M{"$unset": bson.M{"manager_id": ""}}
		_, err = s.ProjectsCollection.UpdateMany(context.Background(), projectFilter, update)
		if err != nil {
			log.Printf("Failed to remove manager %s from projects: %v\n", userID, err)
			return fmt.Errorf("failed to update projects")
		}
	}

	if role == "member" {
		filter := bson.M{"members._id": userID}
		update := bson.M{"$pull": bson.M{"members": bson.M{"_id": userID}}}
		_, err := s.ProjectsCollection.UpdateMany(context.Background(), filter, update)
		if err != nil {
			log.Printf("Failed to remove user %s from projects: %v\n", userID, err)
			return fmt.Errorf("failed to update projects")
		}
	}

	log.Printf("User %s successfully removed from all projects", userID)
	return nil
}

func (s *ProjectService) GetUserProjects(username string) ([]map[string]interface{}, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		return nil, fmt.Errorf("USERS_SERVICE_URL not set")
	}

	// Prvo dohvati ID korisnika
	resp, err := http.Get(fmt.Sprintf("%s/api/users/id/%s", usersServiceURL, username))
	if err != nil {
		return nil, fmt.Errorf("failed to contact users-service: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user ID: %v", resp.Status)
	}

	var data struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode user ID response: %v", err)
	}

	userID, err := primitive.ObjectIDFromHex(data.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %v", err)
	}

	// Dohvati ulogu korisnika (ako ti treba da razlikuješ manager/member)
	roleResp, err := http.Get(fmt.Sprintf("%s/api/users/role/%s", usersServiceURL, username))
	if err != nil {
		return nil, fmt.Errorf("failed to get user role: %v", err)
	}
	defer roleResp.Body.Close()

	var roleData struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(roleResp.Body).Decode(&roleData); err != nil {
		return nil, fmt.Errorf("failed to decode role: %v", err)
	}

	var filter bson.M
	if roleData.Role == "manager" {
		filter = bson.M{"manager_id": userID}
	} else {
		filter = bson.M{"members._id": userID}
	}

	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch projects: %v", err)
	}
	defer cursor.Close(context.Background())

	var projects []map[string]interface{}
	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			continue
		}
		projects = append(projects, map[string]interface{}{
			"id":          project.ID.Hex(),
			"name":        project.Name,
			"description": project.Description,
		})
	}

	return projects, nil
}
