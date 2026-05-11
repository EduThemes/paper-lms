package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type OneRosterService struct {
	connRepo       repository.OneRosterConnectionRepository
	syncLogRepo    repository.OneRosterSyncLogRepository
	userRepo       repository.UserRepository
	courseRepo     repository.CourseRepository
	sectionRepo    repository.SectionRepository
	enrollmentRepo repository.EnrollmentRepository
	accountRepo    repository.AccountRepository
	db             *gorm.DB
	httpClient     *http.Client
}

func NewOneRosterService(
	connRepo repository.OneRosterConnectionRepository,
	syncLogRepo repository.OneRosterSyncLogRepository,
	userRepo repository.UserRepository,
	courseRepo repository.CourseRepository,
	sectionRepo repository.SectionRepository,
	enrollmentRepo repository.EnrollmentRepository,
	accountRepo repository.AccountRepository,
	db *gorm.DB,
) *OneRosterService {
	return &OneRosterService{
		connRepo:       connRepo,
		syncLogRepo:    syncLogRepo,
		userRepo:       userRepo,
		courseRepo:     courseRepo,
		sectionRepo:    sectionRepo,
		enrollmentRepo: enrollmentRepo,
		accountRepo:    accountRepo,
		db:             db,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
}

// ---- Connection CRUD ----

func (s *OneRosterService) CreateConnection(ctx context.Context, conn *models.OneRosterConnection) error {
	if conn.Name == "" {
		return errors.New("name is required")
	}
	if conn.BaseURL == "" {
		return errors.New("base_url is required")
	}
	if conn.ClientID == "" {
		return errors.New("client_id is required")
	}
	if conn.ClientSecret == "" {
		return errors.New("client_secret is required")
	}
	if conn.TokenURL == "" {
		return errors.New("token_url is required")
	}
	if conn.WorkflowState == "" {
		conn.WorkflowState = "active"
	}
	if conn.SyncStatus == "" {
		conn.SyncStatus = "idle"
	}
	if conn.AutoSyncInterval < 1 {
		conn.AutoSyncInterval = 24
	}
	return s.connRepo.Create(ctx, conn)
}

func (s *OneRosterService) UpdateConnection(ctx context.Context, conn *models.OneRosterConnection) error {
	return s.connRepo.Update(ctx, conn)
}

func (s *OneRosterService) DeleteConnection(ctx context.Context, id uint) error {
	return s.connRepo.Delete(ctx, id)
}

func (s *OneRosterService) ListConnections(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterConnection], error) {
	return s.connRepo.ListByAccountID(ctx, accountID, params)
}

func (s *OneRosterService) GetConnection(ctx context.Context, id uint) (*models.OneRosterConnection, error) {
	return s.connRepo.FindByID(ctx, id)
}

func (s *OneRosterService) TestConnection(ctx context.Context, connectionID uint) (bool, string, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID)
	if err != nil {
		return false, "", fmt.Errorf("connection not found: %w", err)
	}

	token, err := s.fetchToken(conn)
	if err != nil {
		return false, fmt.Sprintf("OAuth2 token request failed: %v", err), nil
	}

	orgs, err := s.fetchOrgs(conn.BaseURL, token)
	if err != nil {
		return false, fmt.Sprintf("Failed to fetch orgs: %v", err), nil
	}

	return true, fmt.Sprintf("Connection successful. Found %d organization(s).", len(orgs)), nil
}

// ---- Sync operations ----

func (s *OneRosterService) SyncFull(ctx context.Context, connectionID uint) (*models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}

	if conn.SyncStatus == "syncing" {
		return nil, errors.New("a sync is already in progress for this connection")
	}

	// Mark as syncing
	conn.SyncStatus = "syncing"
	conn.LastSyncError = ""
	if err := s.connRepo.Update(ctx, conn); err != nil {
		return nil, err
	}

	now := time.Now()
	syncLog := &models.OneRosterSyncLog{
		ConnectionID: connectionID,
		SyncType:     "full",
		Status:       "running",
		StartedAt:    &now,
	}
	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		return nil, err
	}

	// Run the sync
	s.runSync(ctx, conn, syncLog, "")

	return syncLog, nil
}

func (s *OneRosterService) SyncIncremental(ctx context.Context, connectionID uint) (*models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, fmt.Errorf("connection not found: %w", err)
	}

	if conn.SyncStatus == "syncing" {
		return nil, errors.New("a sync is already in progress for this connection")
	}

	var filter string
	if conn.LastSyncAt != nil {
		filter = fmt.Sprintf("dateLastModified>='%s'", conn.LastSyncAt.Format(time.RFC3339))
	}

	// Mark as syncing
	conn.SyncStatus = "syncing"
	conn.LastSyncError = ""
	if err := s.connRepo.Update(ctx, conn); err != nil {
		return nil, err
	}

	now := time.Now()
	syncLog := &models.OneRosterSyncLog{
		ConnectionID: connectionID,
		SyncType:     "incremental",
		Status:       "running",
		StartedAt:    &now,
	}
	if err := s.syncLogRepo.Create(ctx, syncLog); err != nil {
		return nil, err
	}

	s.runSync(ctx, conn, syncLog, filter)

	return syncLog, nil
}

func (s *OneRosterService) GetSyncLogs(ctx context.Context, connectionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterSyncLog], error) {
	return s.syncLogRepo.ListByConnectionID(ctx, connectionID, params)
}

func (s *OneRosterService) GetSyncStatus(ctx context.Context, connectionID uint) (*models.OneRosterConnection, *models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID)
	if err != nil {
		return nil, nil, err
	}

	latest, _ := s.syncLogRepo.GetLatestByConnectionID(ctx, connectionID)
	return conn, latest, nil
}

// ---- OneRoster REST API client methods (private) ----

type onerosterTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type onerosterOrg struct {
	SourcedID    string `json:"sourcedId"`
	Status       string `json:"status"`
	Name         string `json:"name"`
	Type         string `json:"type"` // school, district, department, etc.
	Identifier   string `json:"identifier"`
	ParentOrgID  string `json:"parent,omitempty"`
}

type onerosterUser struct {
	SourcedID    string              `json:"sourcedId"`
	Status       string              `json:"status"`
	GivenName    string              `json:"givenName"`
	FamilyName   string              `json:"familyName"`
	Email        string              `json:"email"`
	Username     string              `json:"username"`
	Role         string              `json:"role"` // student, teacher, administrator, etc.
	UserIDs      []onerosterUserID   `json:"userIds,omitempty"`
	Orgs         []onerosterOrgRef   `json:"orgs,omitempty"`
}

type onerosterUserID struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type onerosterOrgRef struct {
	SourcedID string `json:"sourcedId"`
	Type      string `json:"type"`
}

type onerosterClass struct {
	SourcedID  string          `json:"sourcedId"`
	Status     string          `json:"status"`
	Title      string          `json:"title"`
	ClassCode  string          `json:"classCode"`
	ClassType  string          `json:"classType"` // homeroom, scheduled
	Course     onerosterCourse `json:"course,omitempty"`
	School     onerosterOrgRef `json:"school,omitempty"`
	Terms      []onerosterTerm `json:"terms,omitempty"`
}

type onerosterCourse struct {
	SourcedID string `json:"sourcedId"`
	Title     string `json:"title"`
}

type onerosterTerm struct {
	SourcedID string `json:"sourcedId"`
	Type      string `json:"type"`
}

type onerosterEnrollment struct {
	SourcedID string          `json:"sourcedId"`
	Status    string          `json:"status"`
	Role      string          `json:"role"` // student, teacher, administrator
	User      onerosterOrgRef `json:"user"`
	Class     onerosterOrgRef `json:"class"`
	School    onerosterOrgRef `json:"school,omitempty"`
}

func (s *OneRosterService) fetchToken(conn *models.OneRosterConnection) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	if conn.Scope != "" {
		data.Set("scope", conn.Scope)
	}

	req, err := http.NewRequest("POST", conn.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(conn.ClientID, conn.ClientSecret)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp onerosterTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

func (s *OneRosterService) fetchPaginated(baseURL, path, token, filter string) ([]json.RawMessage, error) {
	var allItems []json.RawMessage
	limit := 100
	offset := 0

	for {
		u, err := url.Parse(baseURL + path)
		if err != nil {
			return nil, fmt.Errorf("parsing URL: %w", err)
		}

		q := u.Query()
		q.Set("limit", strconv.Itoa(limit))
		q.Set("offset", strconv.Itoa(offset))
		if filter != "" {
			q.Set("filter", filter)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending request to %s: %w", path, err)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("request to %s failed with status %d: %s", path, resp.StatusCode, string(body))
		}

		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		// OneRoster wraps results in a key matching the entity type
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return nil, fmt.Errorf("decoding response wrapper: %w", err)
		}

		// Find the data array — it's the first key that isn't metadata
		var items []json.RawMessage
		for key, value := range wrapper {
			if key == "statusInfoSet" {
				continue
			}
			if err := json.Unmarshal(value, &items); err == nil && len(items) > 0 {
				break
			}
		}

		if len(items) == 0 {
			break
		}

		allItems = append(allItems, items...)

		if len(items) < limit {
			break // Last page
		}

		offset += limit
	}

	return allItems, nil
}

func (s *OneRosterService) fetchOrgs(baseURL, token string) ([]onerosterOrg, error) {
	raw, err := s.fetchPaginated(baseURL, "/ims/oneroster/v1p1/orgs", token, "")
	if err != nil {
		return nil, err
	}

	var orgs []onerosterOrg
	for _, item := range raw {
		var org onerosterOrg
		if err := json.Unmarshal(item, &org); err != nil {
			continue
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (s *OneRosterService) fetchUsers(baseURL, token, filter string) ([]onerosterUser, error) {
	raw, err := s.fetchPaginated(baseURL, "/ims/oneroster/v1p1/users", token, filter)
	if err != nil {
		return nil, err
	}

	var users []onerosterUser
	for _, item := range raw {
		var user onerosterUser
		if err := json.Unmarshal(item, &user); err != nil {
			continue
		}
		users = append(users, user)
	}
	return users, nil
}

func (s *OneRosterService) fetchClasses(baseURL, token, filter string) ([]onerosterClass, error) {
	raw, err := s.fetchPaginated(baseURL, "/ims/oneroster/v1p1/classes", token, filter)
	if err != nil {
		return nil, err
	}

	var classes []onerosterClass
	for _, item := range raw {
		var class onerosterClass
		if err := json.Unmarshal(item, &class); err != nil {
			continue
		}
		classes = append(classes, class)
	}
	return classes, nil
}

func (s *OneRosterService) fetchEnrollments(baseURL, token, filter string) ([]onerosterEnrollment, error) {
	raw, err := s.fetchPaginated(baseURL, "/ims/oneroster/v1p1/enrollments", token, filter)
	if err != nil {
		return nil, err
	}

	var enrollments []onerosterEnrollment
	for _, item := range raw {
		var enrollment onerosterEnrollment
		if err := json.Unmarshal(item, &enrollment); err != nil {
			continue
		}
		enrollments = append(enrollments, enrollment)
	}
	return enrollments, nil
}

// ---- Sync runner ----

func (s *OneRosterService) runSync(ctx context.Context, conn *models.OneRosterConnection, syncLog *models.OneRosterSyncLog, filter string) {
	var errDetails []string

	defer func() {
		now := time.Now()
		syncLog.CompletedAt = &now

		if len(errDetails) > 0 {
			syncLog.Status = "failed"
			syncLog.Errors = len(errDetails)
			detailsJSON, _ := json.Marshal(errDetails)
			syncLog.ErrorDetails = string(detailsJSON)
			conn.SyncStatus = "error"
			conn.LastSyncError = errDetails[0]
		} else {
			syncLog.Status = "completed"
			conn.SyncStatus = "completed"
			conn.LastSyncError = ""
		}

		conn.LastSyncAt = &now
		_ = s.syncLogRepo.Update(ctx, syncLog)
		_ = s.connRepo.Update(ctx, conn)
	}()

	// 1. Fetch OAuth2 token
	token, err := s.fetchToken(conn)
	if err != nil {
		errDetails = append(errDetails, fmt.Sprintf("token fetch failed: %v", err))
		return
	}

	// 2. Sync orgs
	orgs, err := s.fetchOrgs(conn.BaseURL, token)
	if err != nil {
		errDetails = append(errDetails, fmt.Sprintf("orgs fetch failed: %v", err))
	} else {
		created, updated, errs := s.syncOrgs(ctx, conn.AccountID, orgs)
		syncLog.OrgsCreated = created
		syncLog.OrgsUpdated = updated
		errDetails = append(errDetails, errs...)
	}

	// 3. Sync users
	users, err := s.fetchUsers(conn.BaseURL, token, filter)
	if err != nil {
		errDetails = append(errDetails, fmt.Sprintf("users fetch failed: %v", err))
	} else {
		created, updated, errs := s.syncUsers(ctx, users)
		syncLog.UsersCreated = created
		syncLog.UsersUpdated = updated
		errDetails = append(errDetails, errs...)
	}

	// 4. Sync classes -> Course + CourseSection
	classes, err := s.fetchClasses(conn.BaseURL, token, filter)
	if err != nil {
		errDetails = append(errDetails, fmt.Sprintf("classes fetch failed: %v", err))
	} else {
		created, updated, errs := s.syncClasses(ctx, conn.AccountID, classes)
		syncLog.ClassesCreated = created
		syncLog.ClassesUpdated = updated
		errDetails = append(errDetails, errs...)
	}

	// 5. Sync enrollments
	enrollments, err := s.fetchEnrollments(conn.BaseURL, token, filter)
	if err != nil {
		errDetails = append(errDetails, fmt.Sprintf("enrollments fetch failed: %v", err))
	} else {
		created, updated, errs := s.syncEnrollments(ctx, enrollments)
		syncLog.EnrollmentsCreated = created
		syncLog.EnrollmentsUpdated = updated
		errDetails = append(errDetails, errs...)
	}
}

// ---- Entity mapping ----

func (s *OneRosterService) syncOrgs(ctx context.Context, accountID uint, orgs []onerosterOrg) (int, int, []string) {
	var created, updated int
	var errs []string

	for _, org := range orgs {
		if org.Status == "tobedeleted" {
			continue
		}

		sisID := "oneroster:" + org.SourcedID
		existing, err := s.accountRepo.FindByID(ctx, accountID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to look up account %d: %v", accountID, err))
			continue
		}

		// Update the root account name if it changed
		if existing.Name != org.Name {
			existing.Name = org.Name
			existing.SISAccountID = &sisID
			if err := s.db.WithContext(ctx).Save(existing).Error; err != nil {
				errs = append(errs, fmt.Sprintf("failed to update account for org %s: %v", org.SourcedID, err))
			} else {
				updated++
			}
		}
	}

	return created, updated, errs
}

func (s *OneRosterService) syncUsers(ctx context.Context, users []onerosterUser) (int, int, []string) {
	var created, updated int
	var errs []string

	for _, orUser := range users {
		if orUser.Status == "tobedeleted" {
			continue
		}

		sisID := "oneroster:" + orUser.SourcedID
		name := orUser.GivenName + " " + orUser.FamilyName
		loginID := orUser.Username
		if loginID == "" {
			loginID = orUser.Email
		}
		if loginID == "" {
			errs = append(errs, fmt.Sprintf("user %s has no username or email", orUser.SourcedID))
			continue
		}

		email := orUser.Email
		if email == "" {
			email = loginID + "@placeholder.local"
		}

		role := mapOneRosterRole(orUser.Role)

		existing, _ := s.userRepo.FindBySISUserID(ctx, sisID)
		if existing != nil {
			// Update existing user
			existing.Name = name
			existing.SortableName = orUser.FamilyName + ", " + orUser.GivenName
			existing.ShortName = orUser.GivenName
			existing.Email = email
			existing.Role = role
			if err := s.userRepo.Update(ctx, existing); err != nil {
				errs = append(errs, fmt.Sprintf("failed to update user %s: %v", orUser.SourcedID, err))
			} else {
				updated++
			}
		} else {
			// Create new user
			newUser := &models.User{
				Name:         name,
				SortableName: orUser.FamilyName + ", " + orUser.GivenName,
				ShortName:    orUser.GivenName,
				LoginID:      loginID,
				SISUserID:    &sisID,
				Email:        email,
				Role:         role,
			}
			// Set a random password — user will need to reset or use SSO
			_ = newUser.HashPassword("OneRoster-" + orUser.SourcedID + "-changeme")

			if err := s.userRepo.Create(ctx, newUser); err != nil {
				errs = append(errs, fmt.Sprintf("failed to create user %s: %v", orUser.SourcedID, err))
			} else {
				created++
			}
		}
	}

	return created, updated, errs
}

func (s *OneRosterService) syncClasses(ctx context.Context, accountID uint, classes []onerosterClass) (int, int, []string) {
	var created, updated int
	var errs []string

	for _, class := range classes {
		if class.Status == "tobedeleted" {
			continue
		}

		sisCourseID := "oneroster:" + class.SourcedID
		courseCode := class.ClassCode
		if courseCode == "" {
			courseCode = class.SourcedID
		}

		existing, _ := s.courseRepo.FindBySISCourseID(ctx, sisCourseID)
		if existing != nil {
			existing.Name = class.Title
			existing.CourseCode = courseCode
			if err := s.courseRepo.Update(ctx, existing); err != nil {
				errs = append(errs, fmt.Sprintf("failed to update course for class %s: %v", class.SourcedID, err))
			} else {
				updated++
			}
		} else {
			newCourse := &models.Course{
				AccountID:     accountID,
				Name:          class.Title,
				CourseCode:     courseCode,
				SISCourseID:   &sisCourseID,
				WorkflowState: "available",
			}
			if err := s.courseRepo.Create(ctx, newCourse); err != nil {
				errs = append(errs, fmt.Sprintf("failed to create course for class %s: %v", class.SourcedID, err))
			} else {
				created++

				// Also create a default section for this class
				sisSectionID := "oneroster-section:" + class.SourcedID
				section := &models.CourseSection{
					CourseID:      newCourse.ID,
					Name:          class.Title,
					SISSectionID:  &sisSectionID,
					WorkflowState: "active",
				}
				_ = s.db.WithContext(ctx).Create(section).Error
			}
		}
	}

	return created, updated, errs
}

func (s *OneRosterService) syncEnrollments(ctx context.Context, enrollments []onerosterEnrollment) (int, int, []string) {
	var created, updated int
	var errs []string

	for _, orEnroll := range enrollments {
		if orEnroll.Status == "tobedeleted" {
			continue
		}

		// Look up user by SIS ID
		userSISID := "oneroster:" + orEnroll.User.SourcedID
		user, _ := s.userRepo.FindBySISUserID(ctx, userSISID)
		if user == nil {
			errs = append(errs, fmt.Sprintf("user not found for enrollment %s (user: %s)", orEnroll.SourcedID, orEnroll.User.SourcedID))
			continue
		}

		// Look up course by SIS ID
		courseSISID := "oneroster:" + orEnroll.Class.SourcedID
		course, _ := s.courseRepo.FindBySISCourseID(ctx, courseSISID)
		if course == nil {
			errs = append(errs, fmt.Sprintf("course not found for enrollment %s (class: %s)", orEnroll.SourcedID, orEnroll.Class.SourcedID))
			continue
		}

		enrollType := mapOneRosterEnrollmentRole(orEnroll.Role)

		existing, _ := s.enrollmentRepo.FindByUserAndCourse(ctx, user.ID, course.ID)
		if existing != nil {
			if existing.Type != enrollType {
				existing.Type = enrollType
				existing.Role = enrollType
				if err := s.enrollmentRepo.Update(ctx, existing); err != nil {
					errs = append(errs, fmt.Sprintf("failed to update enrollment %s: %v", orEnroll.SourcedID, err))
				} else {
					updated++
				}
			}
		} else {
			newEnroll := &models.Enrollment{
				UserID:        user.ID,
				CourseID:      course.ID,
				Type:          enrollType,
				Role:          enrollType,
				WorkflowState: "active",
			}
			if err := s.enrollmentRepo.Create(ctx, newEnroll); err != nil {
				errs = append(errs, fmt.Sprintf("failed to create enrollment %s: %v", orEnroll.SourcedID, err))
			} else {
				created++
			}
		}
	}

	return created, updated, errs
}

// ---- Role mapping ----

func mapOneRosterRole(role string) string {
	switch strings.ToLower(role) {
	case "teacher", "aide":
		return "user" // Canvas teacher role
	case "administrator":
		return "admin"
	case "student":
		return "user" // Canvas student role
	default:
		return "user"
	}
}

func mapOneRosterEnrollmentRole(role string) string {
	switch strings.ToLower(role) {
	case "teacher":
		return "TeacherEnrollment"
	case "student":
		return "StudentEnrollment"
	case "administrator":
		return "TeacherEnrollment" // Admins get teacher access to courses
	case "aide":
		return "TaEnrollment"
	case "proctor":
		return "TaEnrollment"
	case "guardian", "parent", "relative":
		return "ObserverEnrollment"
	default:
		return "StudentEnrollment"
	}
}
