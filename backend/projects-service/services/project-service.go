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

	"trello-project/microservices/projects-service/logging"
	"trello-project/microservices/projects-service/models"

	"github.com/sony/gobreaker"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectService struct {
	ProjectsCollection   *mongo.Collection
	HTTPClient           *http.Client
	TasksBreaker         *gobreaker.CircuitBreaker
	UsersBreaker         *gobreaker.CircuitBreaker
	NotificationsBreaker *gobreaker.CircuitBreaker
}

// NewProjectService initializes a new ProjectService with the necessary MongoDB collections.
func NewProjectService(
	projectsCollection *mongo.Collection,
	httpClient *http.Client,
	tasksBreaker *gobreaker.CircuitBreaker,
	usersBreaker *gobreaker.CircuitBreaker,
	notificationsBreaker *gobreaker.CircuitBreaker,

) *ProjectService {
	return &ProjectService{
		ProjectsCollection:   projectsCollection,
		HTTPClient:           httpClient,
		TasksBreaker:         tasksBreaker,
		UsersBreaker:         usersBreaker,
		NotificationsBreaker: notificationsBreaker,
	}
}

// CreateProject creates a new project with the specified parameters.
func (s *ProjectService) CreateProject(name string, description string, expectedEndDate time.Time, minMembers, maxMembers int, managerID primitive.ObjectID) (*models.Project, error) {
	var existingProject models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"name": name}).Decode(&existingProject)
	if err == nil {
		logging.Logger.Warnf("Attempted to create project with existing name: %s", name)
		return nil, errors.New("project with the same name already exists")
	}

	if err != mongo.ErrNoDocuments {
		logging.Logger.Errorf("Database error when checking for existing project '%s': %v", name, err)
		return nil, fmt.Errorf("database error: %v", err)
	}
	if minMembers < 1 || maxMembers < minMembers {
		logging.Logger.Warnf("Invalid member constraints for new project '%s': minMembers=%d, maxMembers=%d", name, minMembers, maxMembers)
		return nil, fmt.Errorf("invalid member constraints: minMembers=%d, maxMembers=%d", minMembers, maxMembers)
	}
	if expectedEndDate.Before(time.Now()) {
		logging.Logger.Warnf("Invalid expected end date for new project '%s': %v (before now)", name, expectedEndDate)
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
		logging.Logger.Errorf("Failed to insert new project '%s' into database: %v", name, err)
		return nil, fmt.Errorf("failed to create project: %v", err)
	}

	project.ID = result.InsertedID.(primitive.ObjectID)
	logging.Logger.Infof("New project '%s' created successfully with ID: %s", project.Name, project.ID.Hex())
	return project, nil
}

// AddMembersToProject adds multiple members to a project.
func (s *ProjectService) AddMembersToProject(projectID primitive.ObjectID, usernames []string) error {
	var project models.Project
	err := s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": projectID}).Decode(&project)
	if err != nil {
		logging.Logger.Errorf("Error finding project %s: %v", projectID.Hex(), err)
		return fmt.Errorf("project not found: %v", err)
	}
	logging.Logger.Infof("Attempting to add members %v to project %s", usernames, projectID.Hex())

	if len(project.Members)+len(usernames) > project.MaxMembers {
		logging.Logger.Warnf("Maximum number of members reached for project %s. Current members: %d, trying to add: %d, max: %d", projectID.Hex(), len(project.Members), len(usernames), project.MaxMembers)
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
			logging.Logger.Infof("Member %s is already in the project, skipping.", username)
		}
	}

	if len(newUsernames) == 0 {
		logging.Logger.Info("No new members to add.")
		return fmt.Errorf("all provided members are already part of the project")
	}

	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	var members []models.Member

	for _, username := range newUsernames {
		url := fmt.Sprintf("%s/api/users/member/%s", usersServiceURL, username)
		logging.Logger.Infof("Fetching user data from: %s", url)

		userData, err := s.UsersBreaker.Execute(func() (interface{}, error) {
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				logging.Logger.Errorf("Error creating request for user %s: %v", username, err)
				return nil, err
			}

			resp, err := s.HTTPClient.Do(req)
			if err != nil {
				logging.Logger.Errorf("Failed to fetch member %s: %v", username, err)
				return nil, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logging.Logger.Errorf("Failed to fetch member %s, status code: %d", username, resp.StatusCode)
				return nil, fmt.Errorf("failed to fetch member %s", username)
			}

			var user models.Member
			if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
				logging.Logger.Errorf("Failed to decode member %s: %v", username, err)
				return nil, err
			}

			return user, nil
		})

		if err != nil {
			logging.Logger.Warnf("User service unavailable or failed for %s: %v. Skipping user.", username, err)
			continue

		}

		members = append(members, userData.(models.Member))
	}

	if len(members) == 0 {
		logging.Logger.Info("No new members were added due to user service failures.")
		return fmt.Errorf("no new members were added")
	}

	// Provera da li bi se prešao maksimalan broj članova nakon dodavanja ovih koji su uspešno dohvaćeni
	if len(project.Members)+len(members) > project.MaxMembers {
		logging.Logger.Warn("Adding these members would exceed the maximum allowed project members.")
		return fmt.Errorf("adding these members would exceed the project's member limit")
	}

	// Ažuriranje baze sa novim članovima

	filter := bson.M{"_id": projectID}
	update := bson.M{"$push": bson.M{"members": bson.M{"$each": members}}}
	_, err = s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		logging.Logger.Errorf("Error updating project members: %v", err)
		return err
	}

	log.Printf("%d new members were added to project '%s'.", len(members), project.Name)

	for _, member := range members {
		go func(m models.Member) {
			message := fmt.Sprintf("You have been added to the project: %s", project.Name)
			_, err := s.NotificationsBreaker.Execute(func() (interface{}, error) {
				return nil, s.sendNotification(m, message)
			})
			if err != nil {
				log.Printf("Failed to send notification to %s: %v", m.Username, err)
			}
		}(member)
	}

	logging.Logger.Info("Members successfully added to the project.")

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
		logging.Logger.Errorf("Error marshaling notification data: %v", err)
		return nil
	}

	notificationURL := os.Getenv("NOTIFICATIONS_SERVICE_URL")
	if notificationURL == "" {
		logging.Logger.Errorf("Notification service URL is not set in .env")
		return fmt.Errorf("notification service URL is not configured")
	}

	req, err := http.NewRequest("POST", notificationURL, bytes.NewBuffer(notificationData))
	if err != nil {
		logging.Logger.Errorf("Error creating new request: %v", err)
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Role", "manager")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		logging.Logger.Errorf("Error sending HTTP request: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		logging.Logger.Errorf("Failed to create notification, status code: %d", resp.StatusCode)
		return nil
	}

	logging.Logger.Infof("Notification successfully sent for member: %s", member.Username)
	return nil
}

// GetProjectMembers retrieves members of a specific project.
func (s *ProjectService) GetProjectMembers(ctx context.Context, projectID string) ([]bson.M, error) {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Errorf("Invalid project ID format: %v", err)
		return nil, err
	}

	var project struct {
		Members []bson.M `bson:"members"`
	}

	err = s.ProjectsCollection.FindOne(ctx, bson.M{"_id": projectObjectID}).Decode(&project)
	if err != nil {
		logging.Logger.Errorf("Error fetching project members from database: %v", err)
		return nil, err
	}

	return project.Members, nil
}

func (s *ProjectService) GetAllUsers() ([]models.Member, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	url := fmt.Sprintf("%s/api/users/members", usersServiceURL)

	// Circuit breaker execution
	result, err := s.UsersBreaker.Execute(func() (interface{}, error) {
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
	})

	// Fallback ako je circuit breaker otvoren ili poziv nije uspeo
	if err != nil {
		logging.Logger.Warnf("[Fallback] Returning empty user list due to error: %v", err)
		return []models.Member{}, nil
	}

	return result.([]models.Member), nil
}

// RemoveMemberFromProject removes a member from a project if they are not assigned to an in-progress task.
func (s *ProjectService) RemoveMemberFromProject(ctx context.Context, projectID, memberID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Warnf("Invalid project ID format: %v", err)
		return fmt.Errorf("invalid project ID format")
	}

	memberObjectID, err := primitive.ObjectIDFromHex(memberID)
	if err != nil {
		logging.Logger.Warnf("Invalid member ID format: %v", err)
		return fmt.Errorf("invalid member ID format")
	}

	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if taskServiceURL == "" {
		logging.Logger.Warnf("TASKS_SERVICE_URL is not set")
		return fmt.Errorf("task service URL is not configured")
	}

	checkURL := fmt.Sprintf("%s/api/tasks/has-active?projectId=%s&memberId=%s", taskServiceURL, projectID, memberID)

	// Circuit Breaker za task servis
	result, err := s.TasksBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %v", err)
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to task service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("task service returned status %d: %s", resp.StatusCode, string(body))
		}

		var r struct {
			HasActiveTasks bool `json:"hasActiveTasks"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return nil, fmt.Errorf("failed to decode task service response: %v", err)
		}

		return r.HasActiveTasks, nil
	})
	if err != nil {
		logging.Logger.Warnf("Circuit breaker error or fallback triggered: %v", err)
		return fmt.Errorf("could not verify task assignment: %v", err)
	}

	if result.(bool) {
		logging.Logger.Warnf("Cannot remove member assigned to an active task")
		return fmt.Errorf("cannot remove member assigned to an active task")
	}

	// Ako nema aktivnih zadataka, ukloni člana iz projekta
	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"_id": memberObjectID}}}

	resultUpdate, err := s.ProjectsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		logging.Logger.Errorf("Failed to remove member from project: %v", err)
		return fmt.Errorf("failed to remove member from project")
	}

	if resultUpdate.ModifiedCount == 0 {
		logging.Logger.Warnf("Member not found in project or already removed")
		return fmt.Errorf("member not found in project or already removed")
	}

	logging.Logger.Infof("Member successfully removed from project.")
	// ✅ Dohvatanje Member objekta iz user servisa
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		logging.Logger.Warnf("USERS_SERVICE_URL is not set")
	} else {
		memberURL := fmt.Sprintf("%s/api/users/member/id/%s", usersServiceURL, memberID)

		req, err := http.NewRequestWithContext(ctx, "GET", memberURL, nil)
		if err == nil {
			resp, err := s.HTTPClient.Do(req)
			if err == nil {
				defer resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					var member models.Member
					if err := json.NewDecoder(resp.Body).Decode(&member); err == nil {
						go func() {
							message := "You have been removed from a project."
							_, err := s.NotificationsBreaker.Execute(func() (interface{}, error) {
								return nil, s.sendNotification(member, message)
							})
							if err != nil {
								logging.Logger.Errorf("Failed to send removal notification to %s: %v", member.Username, err)
							}
						}()
					} else {
						logging.Logger.Errorf("Failed to decode member response: %v", err)
					}
				} else {
					logging.Logger.Warnf("User service returned status %d when fetching member info", resp.StatusCode)
				}
			} else {
				logging.Logger.Errorf("Failed to fetch member from user service: %v", err)
			}
		} else {
			logging.Logger.Errorf("Failed to create request to user service: %v", err)
		}
	}

	return nil
}

// GetAllProjects - preuzima sve projekte iz kolekcije
func (s *ProjectService) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	cursor, err := s.ProjectsCollection.Find(context.Background(), bson.M{})
	if err != nil {
		logging.Logger.Errorf("Unsuccessful procurement of projects: %v", err)
		return nil, fmt.Errorf("unsuccessful procurement of projects: %v", err)
	}
	defer cursor.Close(context.Background())

	if err = cursor.All(context.Background(), &projects); err != nil {
		logging.Logger.Errorf("Unsuccessful decoding of projects: %v", err)
		return nil, fmt.Errorf("unsuccessful decoding of projects: %v", err)
	}

	return projects, nil
}

func (s *ProjectService) GetProjectByID(projectID string) (*models.Project, error) {
	objectId, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Errorf("Invalid project ID format: %s: %v", projectID, err)
		return nil, fmt.Errorf("invalid project ID format")
	}

	var project models.Project
	err = s.ProjectsCollection.FindOne(context.Background(), bson.M{"_id": objectId}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logging.Logger.Warnf("Project not found: %s", projectID)
			return nil, fmt.Errorf("project not found")
		}
		logging.Logger.Errorf("Error fetching project: %v", err)
		return nil, fmt.Errorf("error fetching project: %v", err)
	}
	return &project, nil
}

func (s *ProjectService) GetTasksForProject(projectID string, role string, authToken string) ([]map[string]interface{}, error) {
	// Uzimamo URL za tasks servis iz okruženja
	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if tasksServiceURL == "" {
		logging.Logger.Errorf("TASKS_SERVICE_URL not set")
		return nil, fmt.Errorf("TASKS_SERVICE_URL not set")
	}

	// Formiramo pun URL za API poziv
	url := fmt.Sprintf("%s/api/tasks/project/%s", tasksServiceURL, projectID)
	logging.Logger.Infof("Fetching tasks from: %s", url)

	// Pozivamo HTTP zahtev unutar circuit breaker-a
	result, err := s.TasksBreaker.Execute(func() (interface{}, error) {
		// Kreiramo novi HTTP GET zahtev
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Postavljamo potrebne header-e, kao što su Role i Authorization token
		req.Header.Set("Role", role)
		req.Header.Set("Authorization", authToken)

		// Šaljemo HTTP zahtev koristeći HTTP klijent iz servisa
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			logging.Logger.Errorf("Failed to fetch tasks for project %s: %v", projectID, err)
			return nil, fmt.Errorf("failed to fetch tasks: %v", err)
		}
		// Obezbeđujemo zatvaranje response body-a nakon što završi obrada
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("Failed to fetch tasks for project %s, status code: %d", projectID, resp.StatusCode)
			return nil, fmt.Errorf("failed to fetch tasks, status code: %d", resp.StatusCode)
		}

		// Dekodiramo JSON odgovor u slice mapa (lista zadataka)
		var tasks []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
			logging.Logger.Errorf("Failed to decode tasks response: %v", err)
			return nil, fmt.Errorf("failed to decode tasks: %v", err)
		}

		// Vraćamo uspešan rezultat kao interface{}
		return tasks, nil
	})

	// Ako je došlo do greške unutar circuit breaker-a (npr. servis ne radi ili je breaker otvoren)
	if err != nil {
		logging.Logger.Warnf("[Fallback] Returning empty tasks list due to error: %v", err)
		return []map[string]interface{}{}, nil
	}

	// Uspešan slučaj: konvertujemo interface{} nazad u []map[string]interface{} i vraćamo
	return result.([]map[string]interface{}), nil
}

func (s *ProjectService) getUserIDByUsername(username string) (primitive.ObjectID, error) {
	// Uzimamo URL users servisa iz okruženja
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		logging.Logger.Errorf("USERS_SERVICE_URL not set")
		return primitive.NilObjectID, fmt.Errorf("USERS_SERVICE_URL not set")
	}

	// Formiramo URL za dohvat ID-a na osnovu korisničkog imena
	url := fmt.Sprintf("%s/api/users/id/%s", usersServiceURL, username)

	// Circuit breaker: pokušavamo poziv
	result, err := s.UsersBreaker.Execute(func() (interface{}, error) {
		// Kreiramo GET zahtev
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Šaljemo zahtev
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to contact users-service: %v", err)
		}
		defer resp.Body.Close()

		// Proveravamo statusni kod
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("users-service returned status: %v", resp.Status)
		}

		// Struktura za parsiranje odgovora
		var data struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode user ID response: %v", err)
		}

		// Konvertujemo ID iz stringa u ObjectID
		userID, err := primitive.ObjectIDFromHex(data.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID format: %v", err)
		}

		// Vraćamo ObjectID kao rezultat
		return userID, nil
	})

	// Ako breaker ne uspe, vraćamo prazni ID i grešku
	if err != nil {
		logging.Logger.Warnf("[Fallback] Could not fetch user ID for '%s': %v", username, err)
		return primitive.NilObjectID, err
	}

	// Uspesan rezultat se type-assert-uje i vraća
	return result.(primitive.ObjectID), nil
}

func (s *ProjectService) getUserRoleByUsername(username string) (string, error) {
	// Dohvatanje baze URL-a users servisa iz okruženja
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		logging.Logger.Errorf("USERS_SERVICE_URL not set")
		return "", fmt.Errorf("USERS_SERVICE_URL not set")
	}

	// Formiranje kompletnog URL-a za poziv ka users servisu
	url := fmt.Sprintf("%s/api/users/role/%s", usersServiceURL, username)

	// Poziv users servisa unutar circuit breakera
	result, err := s.UsersBreaker.Execute(func() (interface{}, error) {
		// Kreiranje HTTP GET zahteva
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Slanje zahteva
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to contact users-service: %v", err)
		}
		defer resp.Body.Close()

		// Provera statusnog koda
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("users-service returned status: %v", resp.Status)
		}

		// Parsiranje JSON odgovora u strukturu
		var data struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode role response: %v", err)
		}

		// Vraćamo korisničku rolu
		return data.Role, nil
	})

	// Ako je došlo do greške (npr. breaker otvoren), logujemo i vraćamo prazan string i grešku
	if err != nil {
		logging.Logger.Warnf("[Fallback] Could not fetch role for user '%s': %v", username, err)
		return "", err
	}

	// Uspešno dobijena rola (type assertion iz interface{})
	return result.(string), nil
}

func (s *ProjectService) GetProjectsByUsername(username string) ([]models.Project, error) {
	var projects []models.Project

	// ✅ Pokušaj da dobaviš userID, u slučaju greške vrati prazan niz
	userID, err := s.getUserIDByUsername(username)
	if err != nil {
		logging.Logger.Warnf("[Fallback] Failed to get user ID for '%s': %v", username, err)
		return []models.Project{}, nil
	}

	// ✅ Pokušaj da dobaviš user rolu, fallback na prazan niz ako ne uspe
	role, err := s.getUserRoleByUsername(username)
	if err != nil {
		logging.Logger.Warnf("[Fallback] Failed to get user role for '%s': %v", username, err)
		return []models.Project{}, nil
	}

	// ✅ Formiraj MongoDB filter na osnovu role
	var filter bson.M
	if role == "manager" {
		filter = bson.M{"manager_id": userID}
	} else {
		filter = bson.M{"members.username": username}
	}

	logging.Logger.Infof("Executing MongoDB query with filter: %v", filter)

	// ✅ Pokreni MongoDB query
	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Error fetching projects from MongoDB: %v", err)
		return []models.Project{}, nil // fallback ako padne upit
	}
	defer cursor.Close(context.Background())

	// ✅ Prođi kroz rezultate kursora
	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			logging.Logger.Errorf("Error decoding project document: %v", err)
			return []models.Project{}, nil // fallback na grešku dekodiranja
		}
		projects = append(projects, project)
	}

	// ✅ Proveri greške kursora
	if err := cursor.Err(); err != nil {
		logging.Logger.Errorf("Cursor error: %v", err)
		return []models.Project{}, nil
	}

	logging.Logger.Infof("Found %d projects for username %s", len(projects), username)
	return projects, nil
}

func (s *ProjectService) DeleteProjectAndTasks(ctx context.Context, projectID string, r *http.Request) error {
	// 1. Validacija i konverzija projectID
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Warnf("Invalid project ID format: %v", projectID)
		return fmt.Errorf("invalid project ID format")
	}

	// 2. Provera postojanja projekta u bazi
	filter := bson.M{"_id": projectObjectID}
	var project bson.M
	err = s.ProjectsCollection.FindOne(ctx, filter).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logging.Logger.Warnf("Project not found: %v", projectID)
			return fmt.Errorf("project not found")
		}
		logging.Logger.Errorf("Failed to fetch project: %v", err)
		return fmt.Errorf("failed to fetch project: %v", err)
	}

	// 3. Priprema URL-a za tasks-service
	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if taskServiceURL == "" {
		logging.Logger.Errorf("TASKS_SERVICE_URL not set")
		return fmt.Errorf("TASKS_SERVICE_URL not set")
	}
	url := fmt.Sprintf("%s/api/tasks/project/%s", taskServiceURL, projectID)

	// 4. Circuit breaker za tasks-service DELETE
	_, err = s.TasksBreaker.Execute(func() (interface{}, error) {
		// Kreiranje HTTP DELETE zahteva
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			logging.Logger.Errorf("Failed to create request to tasks-service: %v", err)
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Prosleđivanje zaglavlja iz originalnog requesta
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("Role", r.Header.Get("Role"))

		// Slanje HTTP zahteva
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			logging.Logger.Errorf("Failed to contact tasks-service: %v", err)
			return nil, fmt.Errorf("failed to contact tasks-service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("Tasks-service returned non-OK status: %v", resp.Status)
			return nil, fmt.Errorf("task service returned error: %v", resp.Status)
		}

		return nil, nil
	})

	// 5. Fallback ako breaker odbije ili task servis padne
	if err != nil {
		logging.Logger.Warnf("[Fallback] Tasks were not deleted due to error: %v", err)
		// Možemo odlučiti da prekinemo ceo proces ili samo logujemo
		// return fmt.Errorf("failed to delete tasks: %v", err)
	}

	// 6. Brisanje projekta iz baze
	_, err = s.ProjectsCollection.DeleteOne(ctx, filter)
	if err != nil {
		logging.Logger.Errorf("Failed to delete project: %v", err)
		return fmt.Errorf("failed to delete project: %v", err)
	}

	logging.Logger.Infof("Successfully deleted project and (if possible) related tasks for ID: %s", projectID)
	return nil
}

func (s *ProjectService) GetAllMembers() ([]models.Member, error) {
	// 1. Učitaj URL users-servisa iz .env fajla
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		logging.Logger.Errorf("USERS_SERVICE_URL is not set in .env file")
		return nil, fmt.Errorf("users-service URL is not configured")
	}

	// 2. Formiraj URL za zahtev
	url := fmt.Sprintf("%s/api/users/members", usersServiceURL)

	// 3. Poziv kroz circuit breaker
	result, err := s.UsersBreaker.Execute(func() (interface{}, error) {
		// 3.1 Kreiraj GET zahtev
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request for users-service: %v", err)
		}

		// 3.2 Pošalji zahtev
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch members from users-service: %v", err)
		}
		defer resp.Body.Close()

		// 3.3 Proveri HTTP status
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("users-service returned non-200 status: %d", resp.StatusCode)
		}

		// 3.4 Dekodiraj odgovor
		var members []models.Member
		if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
			return nil, fmt.Errorf("failed to decode members response: %v", err)
		}

		return members, nil
	})

	// 4. Fallback ako je circuit otvoren ili došlo do greške
	if err != nil {
		logging.Logger.Warnf("[Fallback] Returning empty member list due to error: %v", err)
		return []models.Member{}, nil
	}

	// 5. Konverzija rezultata u očekivani tip
	return result.([]models.Member), nil
}

func (s *ProjectService) AddTaskToProject(projectID string, taskID string) error {
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		logging.Logger.Errorf("Invalid project ID format: %v", err)
		return fmt.Errorf("invalid project ID format: %v", err)
	}

	taskObjectID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		logging.Logger.Errorf("Invalid task ID format: %v", err)
		return fmt.Errorf("invalid task ID format: %v", err)
	}

	logging.Logger.Infof("Received request to add task %s to project %s", taskID, projectID)

	// Ažuriranje projekta dodavanjem ID-ja zadatka
	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$push": bson.M{"taskIDs": taskObjectID}}

	logging.Logger.Debugf("MongoDB filter: %+v", filter)
	logging.Logger.Debugf("MongoDB update: %+v", update)

	result, err := s.ProjectsCollection.UpdateOne(context.Background(), filter, update)
	if err != nil {
		logging.Logger.Errorf("Failed to update project with task ID: %v", err)
		return fmt.Errorf("failed to update project with task ID: %v", err)
	}

	if result.ModifiedCount == 0 {
		logging.Logger.Warnf("No project was updated. Possible that project ID %s does not exist.", projectID)
		return fmt.Errorf("no project found with ID %s", projectID)
	}

	logging.Logger.Infof("Task %s successfully added to project %s", taskID, projectID)
	return nil
}

func (s *ProjectService) RemoveUserFromProjects(userID string, role string, authToken string) error {
	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if tasksServiceURL == "" {
		logging.Logger.Errorf("TASKS_SERVICE_URL not set")
		return fmt.Errorf("TASKS_SERVICE_URL not set")
	}

	if role == "manager" {
		projectFilter := bson.M{"manager_id": userID}
		cursor, err := s.ProjectsCollection.Find(context.Background(), projectFilter)
		if err != nil {
			logging.Logger.Errorf("Error fetching projects for manager %s: %v", userID, err)
			return fmt.Errorf("failed to fetch projects")
		}
		defer cursor.Close(context.Background())

		for cursor.Next(context.Background()) {
			var project models.Project
			if err := cursor.Decode(&project); err != nil {
				logging.Logger.Errorf("Error decoding project: %v", err)
				continue
			}

			url := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", tasksServiceURL, project.ID.Hex())
			result, err := s.TasksBreaker.Execute(func() (interface{}, error) {
				req, err := http.NewRequest("GET", url, nil)
				if err != nil {
					return nil, fmt.Errorf("failed to create request: %v", err)
				}
				req.Header.Set("Authorization", "Bearer "+authToken)

				resp, err := s.HTTPClient.Do(req)
				if err != nil {
					return nil, fmt.Errorf("failed to contact task service: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return nil, fmt.Errorf("task service returned non-OK status for project %s: %s", project.ID.Hex(), resp.Status)
				}

				var r struct {
					HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
					return nil, fmt.Errorf("failed to decode task service response: %v", err)
				}

				return r, nil
			})

			if err != nil {
				logging.Logger.Warnf("Circuit breaker error or fallback triggered for task service: %v", err)
				// Fallback ponašanje: ne dozvoli uklanjanje ako ne može da proveri zadatke
				return fmt.Errorf("failed to check tasks for project %s", project.ID.Hex())
			}

			r, ok := result.(struct{ HasUnfinishedTasks bool })
			if !ok {
				logging.Logger.Errorf("Unexpected result type from circuit breaker: %T", result)
				return fmt.Errorf("unexpected result type from circuit breaker")
			}
			if r.HasUnfinishedTasks {
				logging.Logger.Warnf("Cannot remove manager %s: unfinished tasks in project %s", userID, project.ID.Hex())
				return fmt.Errorf("manager cannot be removed from project %s due to unfinished tasks", project.ID.Hex())
			}

		}

		update := bson.M{"$unset": bson.M{"manager_id": ""}}
		_, err = s.ProjectsCollection.UpdateMany(context.Background(), projectFilter, update)
		if err != nil {
			logging.Logger.Errorf("Failed to remove manager %s from projects: %v", userID, err)
			return fmt.Errorf("failed to update projects")
		}
		logging.Logger.Infof("Manager %s successfully removed from all managed projects.", userID)

	}

	if role == "member" {
		// Fetch member details from users-service
		usersServiceURL := os.Getenv("USERS_SERVICE_URL")
		getMemberURL := fmt.Sprintf("%s/api/users/member/id/%s", usersServiceURL, userID)

		req, err := http.NewRequest("GET", getMemberURL, nil)
		if err != nil {
			logging.Logger.Errorf("Error creating request to users-service: %v", err)
			return fmt.Errorf("failed to fetch user data")
		}
		req.Header.Set("Authorization", "Bearer "+authToken)

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			logging.Logger.Errorf("Error contacting users-service: %v", err)
			return fmt.Errorf("failed to fetch user data")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logging.Logger.Errorf("users-service returned status %d", resp.StatusCode)
			return fmt.Errorf("failed to fetch user data")
		}

		var member models.Member
		if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
			logging.Logger.Errorf("Failed to decode user data: %v", err)
			return fmt.Errorf("invalid user data from users-service")
		}

		objectID, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			log.Printf("Invalid userID format: %v", err)
			return fmt.Errorf("invalid user ID format")
		}

		filter := bson.M{"members._id": objectID}
		update := bson.M{"$pull": bson.M{"members": bson.M{"_id": objectID}}}

		_, err = s.ProjectsCollection.UpdateMany(context.Background(), filter, update)
		if err != nil {
			logging.Logger.Errorf("Failed to remove user %s from projects: %v", userID, err)
			return fmt.Errorf("failed to update projects")
		}

		logging.Logger.Infof("User %s successfully removed from all projects", userID)

		message := "You have been removed from one or more projects."
		_, err = s.NotificationsBreaker.Execute(func() (interface{}, error) {
			return nil, s.sendNotification(member, message)
		})
		if err != nil {
			logging.Logger.Errorf("Failed to send notification to member %s: %v", member.Username, err)
		}
	}

	return nil
}

func (s *ProjectService) GetUserProjects(username string) ([]map[string]interface{}, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		logging.Logger.Errorf("USERS_SERVICE_URL not set")
		return nil, fmt.Errorf("USERS_SERVICE_URL not set")
	}

	// ➤ 1. Dohvati korisnikov ID kroz circuit breaker.
	idURL := fmt.Sprintf("%s/api/users/id/%s", usersServiceURL, username)

	idResult, err := s.UsersBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodGet, idURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create ID request: %v", err)
		}

		resp, err := s.HTTPClient.Do(req)
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

		return data.ID, nil
	})
	if err != nil {
		logging.Logger.Warnf("UsersBreaker error while fetching ID: %v", err)
		return nil, fmt.Errorf("could not fetch user ID: %v", err)
	}

	userIDHex := idResult.(string)
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
		logging.Logger.Errorf("Invalid user ID format from service: %v", err)
		return nil, fmt.Errorf("invalid user ID format: %v", err)
	}

	// ➤ 2. Dohvati korisnikovu rolu kroz circuit breaker
	roleURL := fmt.Sprintf("%s/api/users/role/%s", usersServiceURL, username)

	roleResult, err := s.UsersBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodGet, roleURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create role request: %v", err)
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get user role: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("failed to fetch role: %v", resp.Status)
		}

		var data struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return nil, fmt.Errorf("failed to decode role: %v", err)
		}

		return data.Role, nil
	})
	if err != nil {
		logging.Logger.Warnf("UsersBreaker error while fetching role: %v", err)
		return nil, fmt.Errorf("could not fetch user role: %v", err)
	}

	role := roleResult.(string)

	// ➤ 3. Formiraj filter po roli
	var filter bson.M
	if role == "manager" {
		filter = bson.M{"manager_id": userID}
	} else {
		filter = bson.M{"members._id": userID}
	}

	// ➤ 4. Nađi projekte
	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		logging.Logger.Errorf("Failed to fetch projects from DB: %v", err)
		return nil, fmt.Errorf("failed to fetch projects: %v", err)
	}
	defer cursor.Close(context.Background())

	var projects []map[string]interface{}
	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			logging.Logger.Errorf("Error decoding project document: %v", err)
			continue
		}
		projects = append(projects, map[string]interface{}{
			"id":          project.ID.Hex(),
			"name":        project.Name,
			"description": project.Description,
		})
	}
	logging.Logger.Infof("Successfully retrieved projects for user %s", username)

	return projects, nil
}
