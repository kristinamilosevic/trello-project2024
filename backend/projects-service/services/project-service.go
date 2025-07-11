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
// .
//
//	func NewProjectService(projectsCollection *mongo.Collection, httpClient *http.Client) *ProjectService {
//		return &ProjectService{
//			ProjectsCollection: projectsCollection,
//			HTTPClient:         httpClient,
//			ProjectsBreaker:    newBreaker("ProjectsServiceCB"),
//			TasksBreaker:       newBreaker("TasksServiceCB"),
//			UsersBreaker:       newBreaker("UsersServiceCB"),
//		}
//	}
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

// func newBreaker(name string) *gobreaker.CircuitBreaker {
// 	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
// 		Name:        name,
// 		MaxRequests: 1,
// 		Timeout:     2 * time.Second,
// 		ReadyToTrip: func(counts gobreaker.Counts) bool {
// 			return counts.ConsecutiveFailures > 3
// 		},
// 		OnStateChange: func(name string, from, to gobreaker.State) {
// 			log.Printf("Circuit Breaker '%s' changed from '%s' to '%s'\n", name, from.String(), to.String())
// 		},
// 	})
// }

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

	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if taskServiceURL == "" {
		log.Println("TASKS_SERVICE_URL is not set")
		return fmt.Errorf("task service URL is not configured")
	}

	checkURL := fmt.Sprintf("%s/api/tasks/project/%s/has-unfinished", taskServiceURL, projectID.Hex())

	resultAny, err := s.TasksBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest("GET", checkURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP request: %v", err)
		}

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to contact tasks-service: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("tasks-service returned non-OK status %d: %s", resp.StatusCode, string(body))
		}

		var parsed struct {
			HasUnfinishedTasks bool `json:"hasUnfinishedTasks"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
			return nil, fmt.Errorf("failed to decode response: %v", err)
		}
		return parsed, nil
	})

	if err != nil {
		log.Printf("Task service unavailable or error: %v", err)
		// Konzervativan fallback: ne dodaj članove ako ne znaš da li ima zadatke
		return fmt.Errorf("could not verify if project has unfinished tasks")
	}

	result := resultAny.(struct{ HasUnfinishedTasks bool })
	if !result.HasUnfinishedTasks {
		log.Println("Cannot add members: project has no unfinished tasks")
		return fmt.Errorf("cannot add members to a finished project")
	}

	if !result.HasUnfinishedTasks {
		log.Println("Cannot add members: project has no unfinished tasks")
		return fmt.Errorf("cannot add members to a finished project")
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
	// Slanje notifikacija novim članovima asinhrono sa Circuit Breaker zaštitom
	for _, member := range members {
		message := fmt.Sprintf("You have been added to the project: %s!", project.Name)

		// Pokrećemo go rutinu za svaku notifikaciju
		go func(mem models.Member, msg string) {
			_, err := s.NotificationsBreaker.Execute(func() (interface{}, error) {
				return nil, s.sendNotification(mem, msg)
			})

			if err != nil {
				log.Printf("Failed to send notification to member %s: %v", mem.Username, err)
			}
		}(member, message)
	}

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
		log.Println("[Fallback] Returning empty user list due to error:", err)
		return []models.Member{}, nil // ili eventualno nil, err ako želiš da to propagiraš
	}

	return result.([]models.Member), nil
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
		log.Printf("Circuit breaker error or fallback triggered: %v", err)
		return fmt.Errorf("could not verify task assignment: %v", err)
	}

	if result.(bool) {
		log.Println("Cannot remove member assigned to an active task")
		return fmt.Errorf("cannot remove member assigned to an active task")
	}

	// Ako nema aktivnih zadataka, ukloni člana iz projekta
	filter := bson.M{"_id": projectObjectID}
	update := bson.M{"$pull": bson.M{"members": bson.M{"_id": memberObjectID}}}

	resultUpdate, err := s.ProjectsCollection.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Println("Failed to remove member from project")
		return fmt.Errorf("failed to remove member from project")
	}

	if resultUpdate.ModifiedCount == 0 {
		log.Println("Member not found in project or already removed")
		return fmt.Errorf("member not found in project or already removed")
	}

	log.Println("Member successfully removed from project.")
	// ✅ Dohvatanje Member objekta iz user servisa
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		log.Println("USERS_SERVICE_URL is not set")
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
								log.Printf("Failed to send removal notification to %s: %v", member.Username, err)
							}
						}()
					} else {
						log.Printf("Failed to decode member response: %v", err)
					}
				} else {
					log.Printf("User service returned status %d when fetching member info", resp.StatusCode)
				}
			} else {
				log.Printf("Failed to fetch member from user service: %v", err)
			}
		} else {
			log.Printf("Failed to create request to user service: %v", err)
		}
	}

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
	// Uzimamo URL za tasks servis iz okruženja
	tasksServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if tasksServiceURL == "" {
		return nil, fmt.Errorf("TASKS_SERVICE_URL not set")
	}

	// Formiramo pun URL za API poziv
	url := fmt.Sprintf("%s/api/tasks/project/%s", tasksServiceURL, projectID)
	fmt.Printf("Fetching tasks from: %s\n", url)

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
			log.Printf("Failed to fetch tasks for project %s: %v\n", projectID, err)
			return nil, fmt.Errorf("failed to fetch tasks: %v", err)
		}
		// Obezbeđujemo zatvaranje response body-a nakon što završi obrada
		defer resp.Body.Close()

		// Proveravamo da li je statusni kod 200 OK
		if resp.StatusCode != http.StatusOK {
			log.Printf("Failed to fetch tasks for project %s, status code: %d\n", projectID, resp.StatusCode)
			return nil, fmt.Errorf("failed to fetch tasks, status code: %d", resp.StatusCode)
		}

		// Dekodiramo JSON odgovor u slice mapa (lista zadataka)
		var tasks []map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
			log.Printf("Failed to decode tasks response: %v\n", err)
			return nil, fmt.Errorf("failed to decode tasks: %v", err)
		}

		// Vraćamo uspešan rezultat kao interface{}
		return tasks, nil
	})

	// Ako je došlo do greške unutar circuit breaker-a (npr. servis ne radi ili je breaker otvoren)
	if err != nil {
		log.Printf("[Fallback] Returning empty tasks list due to error: %v\n", err)
		// Vraćamo fallback vrednost - praznu listu zadataka i bez greške (ili možeš i grešku ako želiš da propagiraš)
		return []map[string]interface{}{}, nil
	}

	// Uspešan slučaj: konvertujemo interface{} nazad u []map[string]interface{} i vraćamo
	return result.([]map[string]interface{}), nil
}

func (s *ProjectService) getUserIDByUsername(username string) (primitive.ObjectID, error) {
	// Uzimamo URL users servisa iz okruženja
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
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
		log.Printf("[Fallback] Could not fetch user ID for '%s': %v\n", username, err)
		return primitive.NilObjectID, err
	}

	// Uspesan rezultat se type-assert-uje i vraća
	return result.(primitive.ObjectID), nil
}

func (s *ProjectService) getUserRoleByUsername(username string) (string, error) {
	// Dohvatanje baze URL-a users servisa iz okruženja
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
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
		log.Printf("[Fallback] Could not fetch role for user '%s': %v\n", username, err)
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
		log.Printf("[Fallback] Failed to get user ID for '%s': %v\n", username, err)
		return []models.Project{}, nil
	}

	// ✅ Pokušaj da dobaviš user rolu, fallback na prazan niz ako ne uspe
	role, err := s.getUserRoleByUsername(username)
	if err != nil {
		log.Printf("[Fallback] Failed to get user role for '%s': %v\n", username, err)
		return []models.Project{}, nil
	}

	// ✅ Formiraj MongoDB filter na osnovu role
	var filter bson.M
	if role == "manager" {
		filter = bson.M{"manager_id": userID}
	} else {
		filter = bson.M{"members.username": username}
	}

	log.Printf("Executing MongoDB query with filter: %v", filter)

	// ✅ Pokreni MongoDB query
	cursor, err := s.ProjectsCollection.Find(context.Background(), filter)
	if err != nil {
		log.Printf("Error fetching projects from MongoDB: %v", err)
		return []models.Project{}, nil // fallback ako padne upit
	}
	defer cursor.Close(context.Background())

	// ✅ Prođi kroz rezultate kursora
	for cursor.Next(context.Background()) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			log.Printf("Error decoding project document: %v", err)
			return []models.Project{}, nil // fallback na grešku dekodiranja
		}
		projects = append(projects, project)
	}

	// ✅ Proveri greške kursora
	if err := cursor.Err(); err != nil {
		log.Printf("Cursor error: %v", err)
		return []models.Project{}, nil
	}

	log.Printf("Found %d projects for username %s", len(projects), username)
	return projects, nil
}

func (s *ProjectService) DeleteProjectAndTasks(ctx context.Context, projectID string, r *http.Request) error {
	// 1. Validacija i konverzija projectID
	projectObjectID, err := primitive.ObjectIDFromHex(projectID)
	if err != nil {
		log.Printf("Invalid project ID format: %v", projectID)
		return fmt.Errorf("invalid project ID format")
	}

	// 2. Provera postojanja projekta u bazi
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

	// 3. Priprema URL-a za tasks-service
	taskServiceURL := os.Getenv("TASKS_SERVICE_URL")
	if taskServiceURL == "" {
		return fmt.Errorf("TASKS_SERVICE_URL not set")
	}
	url := fmt.Sprintf("%s/api/tasks/project/%s", taskServiceURL, projectID)

	// 4. Circuit breaker za tasks-service DELETE
	_, err = s.TasksBreaker.Execute(func() (interface{}, error) {
		// Kreiranje HTTP DELETE zahteva
		req, err := http.NewRequest(http.MethodDelete, url, nil)
		if err != nil {
			log.Printf("Failed to create request to tasks-service: %v", err)
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		// Prosleđivanje zaglavlja iz originalnog requesta
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
		req.Header.Set("Role", r.Header.Get("Role"))

		// Slanje HTTP zahteva
		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			log.Printf("Failed to contact tasks-service: %v", err)
			return nil, fmt.Errorf("failed to contact tasks-service: %v", err)
		}
		defer resp.Body.Close()

		// Provera statusnog koda
		if resp.StatusCode != http.StatusOK {
			log.Printf("Tasks-service returned non-OK status: %v", resp.Status)
			return nil, fmt.Errorf("task service returned error: %v", resp.Status)
		}

		return nil, nil
	})

	// 5. Fallback ako breaker odbije ili task servis padne
	if err != nil {
		log.Printf("[Fallback] Tasks were not deleted due to error: %v", err)
		// Možemo odlučiti da prekinemo ceo proces ili samo logujemo
		// return fmt.Errorf("failed to delete tasks: %v", err)
	}

	// 6. Brisanje projekta iz baze
	_, err = s.ProjectsCollection.DeleteOne(ctx, filter)
	if err != nil {
		log.Printf("Failed to delete project: %v", err)
		return fmt.Errorf("failed to delete project: %v", err)
	}

	log.Printf("Successfully deleted project and (if possible) related tasks for ID: %s", projectID)
	return nil
}

func (s *ProjectService) GetAllMembers() ([]models.Member, error) {
	// 1. Učitaj URL users-servisa iz .env fajla
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
		log.Println("USERS_SERVICE_URL is not set in .env file")
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
		log.Printf("[Fallback] Returning empty member list due to error: %v", err)
		return []models.Member{}, nil // ili: return nil, err ako želiš da propagiraš grešku
	}

	// 5. Konverzija rezultata u očekivani tip
	return result.([]models.Member), nil
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
				log.Printf("Circuit breaker error or fallback triggered for task service: %v", err)
				// Fallback ponašanje: ne dozvoli uklanjanje ako ne može da proveri zadatke
				return fmt.Errorf("failed to check tasks for project %s", project.ID.Hex())
			}

			r, ok := result.(struct{ HasUnfinishedTasks bool })
			if !ok {
				return fmt.Errorf("unexpected result type from circuit breaker")
			}
			if r.HasUnfinishedTasks {
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
		// Fetch member details from users-service
		usersServiceURL := os.Getenv("USERS_SERVICE_URL")
		getMemberURL := fmt.Sprintf("%s/api/users/id/%s", usersServiceURL, userID)

		req, err := http.NewRequest("GET", getMemberURL, nil)
		if err != nil {
			log.Printf("Error creating request to users-service: %v", err)
			return fmt.Errorf("failed to fetch user data")
		}
		req.Header.Set("Authorization", "Bearer "+authToken)

		resp, err := s.HTTPClient.Do(req)
		if err != nil {
			log.Printf("Error contacting users-service: %v", err)
			return fmt.Errorf("failed to fetch user data")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("users-service returned status %d", resp.StatusCode)
			return fmt.Errorf("failed to fetch user data")
		}

		var member models.Member
		if err := json.NewDecoder(resp.Body).Decode(&member); err != nil {
			log.Printf("Failed to decode user data: %v", err)
			return fmt.Errorf("invalid user data from users-service")
		}

		filter := bson.M{"members._id": userID}
		update := bson.M{"$pull": bson.M{"members": bson.M{"_id": userID}}}
		_, err = s.ProjectsCollection.UpdateMany(context.Background(), filter, update)
		if err != nil {
			log.Printf("Failed to remove user %s from projects: %v\n", userID, err)
			return fmt.Errorf("failed to update projects")
		}

		log.Printf("User %s successfully removed from all projects", userID)

		// Notifikacija samo za member-e
		message := "You have been removed from one or more projects."
		_, err = s.NotificationsBreaker.Execute(func() (interface{}, error) {
			return nil, s.sendNotification(member, message)
		})
		if err != nil {
			log.Printf("Failed to send notification to member %s: %v", member.Username, err)
		}
	}

	return nil
}

func (s *ProjectService) GetUserProjects(username string) ([]map[string]interface{}, error) {
	usersServiceURL := os.Getenv("USERS_SERVICE_URL")
	if usersServiceURL == "" {
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
		log.Printf("UsersBreaker error while fetching ID: %v", err)
		return nil, fmt.Errorf("could not fetch user ID: %v", err)
	}

	userIDHex := idResult.(string)
	userID, err := primitive.ObjectIDFromHex(userIDHex)
	if err != nil {
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
		log.Printf("UsersBreaker error while fetching role: %v", err)
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
