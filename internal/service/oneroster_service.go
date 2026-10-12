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

	"github.com/EduThemes/paper-lms/internal/auth/initialpassword"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/security"
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
		// SafeTransport re-validates the resolved IP at connect time
		// (DNS-rebinding defense) on top of the up-front ValidateExternalURL.
		httpClient:     &http.Client{Timeout: 30 * time.Second, Transport: security.SafeTransport()},
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

func (s *OneRosterService) GetConnection(ctx context.Context, id, accountID uint) (*models.OneRosterConnection, error) {
	return s.connRepo.FindByID(ctx, id, accountID)
}

func (s *OneRosterService) TestConnection(ctx context.Context, connectionID, accountID uint) (bool, string, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID, accountID)
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

func (s *OneRosterService) SyncFull(ctx context.Context, connectionID, accountID uint) (*models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID, accountID)
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
		return nil, s.releaseSyncStart(ctx, conn, err)
	}

	// Run the sync
	s.runSync(ctx, conn, syncLog, "")

	return syncLog, nil
}

func (s *OneRosterService) SyncIncremental(ctx context.Context, connectionID, accountID uint) (*models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID, accountID)
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
		return nil, s.releaseSyncStart(ctx, conn, err)
	}

	s.runSync(ctx, conn, syncLog, filter)

	return syncLog, nil
}

// releaseSyncStart undoes the "syncing" mark when a sync fails before
// runSync (whose deferred block normally clears it) gets to run. Without
// it the connection stayed "syncing" forever: every later manual sync
// was refused as "already in progress" and auto-sync never found it due.
func (s *OneRosterService) releaseSyncStart(ctx context.Context, conn *models.OneRosterConnection, cause error) error {
	conn.SyncStatus = "error"
	conn.LastSyncError = fmt.Sprintf("could not start sync: %v", cause)
	_ = s.connRepo.Update(ctx, conn)
	return cause
}

// autoSyncDue reports whether an auto-sync connection is due at now: it
// has never synced, or its last sync is at least AutoSyncInterval hours
// old (default 24). A connection already syncing is never due.
func autoSyncDue(conn *models.OneRosterConnection, now time.Time) bool {
	if conn.SyncStatus == "syncing" {
		return false
	}
	if conn.LastSyncAt == nil {
		return true
	}
	interval := conn.AutoSyncInterval
	if interval < 1 {
		interval = 24
	}
	return now.Sub(*conn.LastSyncAt) >= time.Duration(interval)*time.Hour
}

// RunDueAutoSyncs runs a full sync for every auto_sync connection whose
// interval has elapsed (B173: the auto-sync toggle previously did
// nothing — no job ever read it). Full, not incremental, so the opt-in
// deprovision passes see the whole roster. The scheduler calls this
// hourly; one failing connection does not stop the others.
func (s *OneRosterService) RunDueAutoSyncs(ctx context.Context, now time.Time) (int, error) {
	conns, err := s.connRepo.ListAutoSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing auto-sync connections: %w", err)
	}
	ran := 0
	var errs []string
	for i := range conns {
		conn := &conns[i]
		if !autoSyncDue(conn, now) {
			continue
		}
		if _, err := s.SyncFull(ctx, conn.ID, conn.AccountID); err != nil {
			errs = append(errs, fmt.Sprintf("connection %d: %v", conn.ID, err))
			continue
		}
		ran++
	}
	if len(errs) > 0 {
		return ran, fmt.Errorf("auto-sync errors: %s", strings.Join(errs, "; "))
	}
	return ran, nil
}

func (s *OneRosterService) GetSyncLogs(ctx context.Context, connectionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterSyncLog], error) {
	return s.syncLogRepo.ListByConnectionID(ctx, connectionID, params)
}

func (s *OneRosterService) GetSyncStatus(ctx context.Context, connectionID, accountID uint) (*models.OneRosterConnection, *models.OneRosterSyncLog, error) {
	conn, err := s.connRepo.FindByID(ctx, connectionID, accountID)
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
	// Agents links students and their parents/guardians (OneRoster 1.1
	// users.agents); each side lists the other.
	Agents []onerosterOrgRef `json:"agents,omitempty"`
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
	// SECURITY (F-023): SSRF defense on the OAuth token URL. Admin-
	// controlled per OneRosterConnection row. Combined with cross-
	// tenant admin escalation (F-001 pre-fix), an attacker could
	// install a OneRoster connection pointing at internal services
	// and the server would POST basic-auth credentials to it.
	if err := security.ValidateExternalURL(context.Background(), conn.TokenURL); err != nil {
		return "", fmt.Errorf("OneRoster token URL rejected by SSRF guard: %w", err)
	}

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
	defer func() { _ = resp.Body.Close() }()

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

		// SECURITY (F-023): same defense as fetchToken. Validate on
		// each loop iteration is cheap (DNS lookup cached by the
		// resolver) and pins the contract — the base URL stays public
		// for every page.
		if err := security.ValidateExternalURL(context.Background(), u.String()); err != nil {
			return nil, fmt.Errorf("OneRoster base URL rejected by SSRF guard: %w", err)
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
		_ = resp.Body.Close()

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
	users, usersErr := s.fetchUsers(conn.BaseURL, token, filter)
	if usersErr != nil {
		errDetails = append(errDetails, fmt.Sprintf("users fetch failed: %v", usersErr))
	} else {
		created, updated, errs := s.syncUsers(ctx, conn.AccountID, users)
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
		res := s.syncEnrollments(ctx, conn.AccountID, enrollments)
		syncLog.EnrollmentsCreated = res.created
		syncLog.EnrollmentsUpdated = res.updated
		errDetails = append(errDetails, res.errs...)

		// Enrollments the full roster no longer lists are dropped
		// (B174) under the same opt-in, full-sync-only and
		// circuit-breaker guardrails as the user deprovision pass.
		if conn.DeprovisionEnabled && syncLog.SyncType == "full" {
			removed, abortReason, errs := s.removeAbsentEnrollments(ctx, conn.AccountID, res.present)
			syncLog.EnrollmentsUpdated += removed
			errDetails = append(errDetails, errs...)
			if abortReason != "" {
				syncLog.DeprovisionAbortedReason = abortReason
			}
		}
	}

	// 6. Deprovision pass — opt-in, FULL syncs only (an incremental
	// sync is a delta; absence from it means nothing), and only when
	// the roster fetch above succeeded so "absent" reflects the real
	// roster rather than a failed request.
	if conn.DeprovisionEnabled && syncLog.SyncType == "full" && usersErr == nil {
		result, err := s.reconcileDeprovision(ctx, conn, onerosterPresentSet(users), true)
		if err != nil {
			errDetails = append(errDetails, fmt.Sprintf("deprovision pass failed: %v", err))
		} else if result.Aborted {
			if syncLog.DeprovisionAbortedReason != "" {
				syncLog.DeprovisionAbortedReason = result.AbortReason + "; " + syncLog.DeprovisionAbortedReason
			} else {
				syncLog.DeprovisionAbortedReason = result.AbortReason
			}
		} else {
			syncLog.UsersDeprovisioned = len(result.ToSuspend)
			syncLog.UsersReactivated = len(result.ToReactivate)
		}
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

func (s *OneRosterService) syncUsers(ctx context.Context, accountID uint, users []onerosterUser) (int, int, []string) {
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
		if existing != nil && existing.AccountID != accountID {
			// SIS ids are deployment-wide; never touch another tenant's user.
			errs = append(errs, fmt.Sprintf("user %s belongs to another account", orUser.SourcedID))
			continue
		}
		if existing != nil {
			// Update existing user. Role is deliberately NOT synced: the
			// account-level role is managed in Paper (B074).
			existing.Name = name
			existing.SortableName = orUser.FamilyName + ", " + orUser.GivenName
			existing.ShortName = orUser.GivenName
			existing.Email = email
			if err := s.userRepo.Update(ctx, existing); err != nil {
				errs = append(errs, fmt.Sprintf("failed to update user %s: %v", orUser.SourcedID, err))
			} else {
				updated++
			}
		} else {
			// Create new user. AccountID must be the connection's tenant:
			// User.BeforeCreate defaults a zero AccountID to 1, which
			// would both leak the user into the root account and make
			// the account-scoped deprovision pass blind to them.
			newUser := &models.User{
				AccountID:    accountID,
				Name:         name,
				SortableName: orUser.FamilyName + ", " + orUser.GivenName,
				ShortName:    orUser.GivenName,
				LoginID:      loginID,
				SISUserID:    &sisID,
				Email:        email,
				Role:         role,
			}
			// Set a cryptographically random initial password. The plaintext
			// is irrecoverable; the user MUST authenticate via SSO or use
			// the password-reset flow before they can log in. See
			// internal/auth/initialpassword for the operator-side contract.
			// Prior code derived the password from the public sourcedID
			// ("OneRoster-" + SourcedID + "-changeme"), which was
			// trivially recoverable by anyone with the SIS roster.
			initialPW, pwErr := initialpassword.GenerateInitialPassword()
			if pwErr != nil {
				errs = append(errs, fmt.Sprintf("failed to generate initial password for %s: %v", orUser.SourcedID, pwErr))
				continue
			}
			if hashErr := newUser.HashPassword(initialPW); hashErr != nil {
				errs = append(errs, fmt.Sprintf("failed to hash initial password for %s: %v", orUser.SourcedID, hashErr))
				continue
			}
			// Wave 1.6 follow-up: the random initial password is
			// irrecoverable, so flag the row so the LoginPipeline
			// gates session minting and forces a real password
			// before the user gets any session.
			newUser.RequiresPasswordReset = true

			if err := s.userRepo.Create(ctx, newUser); err != nil {
				errs = append(errs, fmt.Sprintf("failed to create user %s: %v", orUser.SourcedID, err))
			} else {
				created++
			}
		}
	}

	errs = append(errs, s.syncGuardianLinks(ctx, accountID, users)...)

	return created, updated, errs
}

// syncGuardianLinks turns the roster's student <-> parent/guardian
// agents relationships into durable ObserverLinks (B174), so a guardian
// is linked to their child and observes the child's classes. Linking
// mirrors the child's current enrollments immediately; later changes
// follow through Enrollment.AfterSave.
func (s *OneRosterService) syncGuardianLinks(ctx context.Context, accountID uint, users []onerosterUser) []string {
	type pair struct{ guardian, student string }
	var pairs []pair
	for _, u := range users {
		if u.Status == "tobedeleted" || len(u.Agents) == 0 {
			continue
		}
		role := strings.ToLower(u.Role)
		for _, a := range u.Agents {
			switch {
			case role == "student":
				pairs = append(pairs, pair{guardian: a.SourcedID, student: u.SourcedID})
			case isOneRosterGuardianRole(role):
				pairs = append(pairs, pair{guardian: u.SourcedID, student: a.SourcedID})
			}
		}
	}
	if len(pairs) == 0 || s.db == nil {
		return nil
	}

	links := postgres.NewObserverLinkRepository(s.db)
	seen := map[pair]bool{}
	var errs []string
	for _, p := range pairs {
		if seen[p] {
			continue
		}
		seen[p] = true
		guardian := s.findOneRosterUser(ctx, accountID, p.guardian)
		student := s.findOneRosterUser(ctx, accountID, p.student)
		if guardian == nil || student == nil {
			// The other side may arrive in a later page or sync.
			continue
		}
		created, err := links.Activate(ctx, guardian.ID, student.ID, accountID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("failed to link guardian %s to student %s: %v", p.guardian, p.student, err))
			continue
		}
		if created {
			if err := links.SyncStudent(ctx, student.ID); err != nil {
				errs = append(errs, fmt.Sprintf("failed to sync guardian %s access: %v", p.guardian, err))
			}
		}
	}
	return errs
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
		if existing != nil && existing.AccountID != accountID {
			// SIS ids are deployment-wide; never touch another tenant's course.
			errs = append(errs, fmt.Sprintf("class %s belongs to another account", class.SourcedID))
			continue
		}
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
				WorkflowState: models.CourseAvailable,
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

// onerosterEnrollmentKey identifies a roster enrollment once resolved
// to Paper ids.
type onerosterEnrollmentKey struct {
	userID, courseID uint
	enrollType       string
}

// onerosterEnrollmentSyncResult carries the counters and the set of
// enrollments the roster asserts, for the absence pass.
type onerosterEnrollmentSyncResult struct {
	created, updated int
	errs             []string
	present          map[onerosterEnrollmentKey]bool
}

// findOneRosterUser / findOneRosterCourse resolve a sourcedId inside the
// connection's account only; SIS ids are deployment-wide, so a match in
// another tenant is treated as "not found".
func (s *OneRosterService) findOneRosterUser(ctx context.Context, accountID uint, sourcedID string) *models.User {
	u, _ := s.userRepo.FindBySISUserID(ctx, onerosterSISPrefix+sourcedID)
	if u == nil || u.AccountID != accountID {
		return nil
	}
	return u
}

func (s *OneRosterService) findOneRosterCourse(ctx context.Context, accountID uint, sourcedID string) *models.Course {
	c, _ := s.courseRepo.FindBySISCourseID(ctx, onerosterSISPrefix+sourcedID)
	if c == nil || c.AccountID != accountID {
		return nil
	}
	return c
}

// findRosterEnrollment returns the enrollment a roster row maps onto, in
// any workflow state: the same-type row if there is one (preferring a
// non-deleted row), otherwise — for a role change — any non-observer
// row of the user in the course.
func (s *OneRosterService) findRosterEnrollment(ctx context.Context, userID, courseID uint, enrollType string) *models.Enrollment {
	var e models.Enrollment
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND type = ? AND associated_user_id IS NULL", userID, courseID, enrollType).
		Order("(workflow_state = 'deleted') ASC, id ASC").First(&e).Error
	if err == nil {
		return &e
	}
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND type <> ?", userID, courseID, "ObserverEnrollment").
		Order("(workflow_state = 'deleted') ASC, id ASC").First(&e).Error
	if err == nil {
		return &e
	}
	return nil
}

func isOneRosterGuardianRole(role string) bool {
	switch strings.ToLower(role) {
	case "guardian", "parent", "relative":
		return true
	}
	return false
}

func (s *OneRosterService) syncEnrollments(ctx context.Context, accountID uint, enrollments []onerosterEnrollment) onerosterEnrollmentSyncResult {
	res := onerosterEnrollmentSyncResult{present: map[onerosterEnrollmentKey]bool{}}

	// Guardians are handled after students: their observer enrollments
	// hang off the child's StudentEnrollment (see syncGuardianEnrollment).
	var guardians []onerosterEnrollment

	for _, orEnroll := range enrollments {
		if isOneRosterGuardianRole(orEnroll.Role) {
			guardians = append(guardians, orEnroll)
			continue
		}

		user := s.findOneRosterUser(ctx, accountID, orEnroll.User.SourcedID)
		if user == nil {
			res.errs = append(res.errs, fmt.Sprintf("user not found for enrollment %s (user: %s)", orEnroll.SourcedID, orEnroll.User.SourcedID))
			continue
		}
		course := s.findOneRosterCourse(ctx, accountID, orEnroll.Class.SourcedID)
		if course == nil {
			res.errs = append(res.errs, fmt.Sprintf("course not found for enrollment %s (class: %s)", orEnroll.SourcedID, orEnroll.Class.SourcedID))
			continue
		}

		enrollType := mapOneRosterEnrollmentRole(orEnroll.Role)
		existing := s.findRosterEnrollment(ctx, user.ID, course.ID, enrollType)

		// The SIS marked the enrollment for removal (e.g. the student
		// switched class periods): drop it. Saving through the repo
		// lets Enrollment.AfterSave drop linked observers' access too.
		if orEnroll.Status == "tobedeleted" {
			if existing != nil && existing.Type == enrollType && existing.WorkflowState != models.EnrollmentDeleted {
				existing.WorkflowState = models.EnrollmentDeleted
				if err := s.enrollmentRepo.Update(ctx, existing); err != nil {
					res.errs = append(res.errs, fmt.Sprintf("failed to remove enrollment %s: %v", orEnroll.SourcedID, err))
				} else {
					res.updated++
				}
			}
			continue
		}

		res.present[onerosterEnrollmentKey{user.ID, course.ID, enrollType}] = true

		if existing != nil {
			if existing.Type != enrollType || existing.WorkflowState != models.EnrollmentActive {
				existing.Type = enrollType
				existing.Role = enrollType
				existing.WorkflowState = models.EnrollmentActive
				if err := s.enrollmentRepo.Update(ctx, existing); err != nil {
					res.errs = append(res.errs, fmt.Sprintf("failed to update enrollment %s: %v", orEnroll.SourcedID, err))
				} else {
					res.updated++
				}
			}
			continue
		}

		newEnroll := &models.Enrollment{
			UserID:        user.ID,
			CourseID:      course.ID,
			Type:          enrollType,
			Role:          enrollType,
			WorkflowState: models.EnrollmentActive,
		}
		if err := s.enrollmentRepo.Create(ctx, newEnroll); err != nil {
			res.errs = append(res.errs, fmt.Sprintf("failed to create enrollment %s: %v", orEnroll.SourcedID, err))
		} else {
			res.created++
		}
	}

	for _, g := range guardians {
		if g.Status == "tobedeleted" {
			// Guardian access follows the child's enrollment and the
			// guardian link; nothing to do per class.
			continue
		}
		if err := s.syncGuardianEnrollment(ctx, accountID, g); err != nil {
			res.errs = append(res.errs, err.Error())
		}
	}

	return res
}

// syncGuardianEnrollment gives a guardian observer access to a class
// through the child they are linked to (users' agents relationship,
// stored as ObserverLinks by syncUsers). Previously the guardian got an
// ObserverEnrollment with no AssociatedUserID, i.e. no link to any
// child. Now each linked child enrolled in the class gets an associated
// observer enrollment via the same mirror Enrollment.AfterSave uses.
func (s *OneRosterService) syncGuardianEnrollment(ctx context.Context, accountID uint, g onerosterEnrollment) error {
	guardian := s.findOneRosterUser(ctx, accountID, g.User.SourcedID)
	if guardian == nil {
		return fmt.Errorf("user not found for enrollment %s (user: %s)", g.SourcedID, g.User.SourcedID)
	}
	course := s.findOneRosterCourse(ctx, accountID, g.Class.SourcedID)
	if course == nil {
		return fmt.Errorf("course not found for enrollment %s (class: %s)", g.SourcedID, g.Class.SourcedID)
	}

	var students []models.Enrollment
	if err := s.db.WithContext(ctx).
		Where("course_id = ? AND type = ? AND user_id IN (SELECT student_id FROM observer_links WHERE observer_id = ? AND workflow_state = ?)",
			course.ID, "StudentEnrollment", guardian.ID, models.ObserverLinkActive).
		Find(&students).Error; err != nil {
		return fmt.Errorf("failed to resolve students for guardian enrollment %s: %v", g.SourcedID, err)
	}
	if len(students) == 0 {
		return fmt.Errorf("guardian enrollment %s: guardian %s has no linked student in class %s", g.SourcedID, g.User.SourcedID, g.Class.SourcedID)
	}
	for i := range students {
		if err := models.SyncObserverEnrollmentsForStudent(s.db.WithContext(ctx), &students[i]); err != nil {
			return fmt.Errorf("failed to sync guardian enrollment %s: %v", g.SourcedID, err)
		}
	}
	return nil
}

// removeAbsentEnrollments is the enrollment half of roster-driven
// deprovisioning (B174): on a full sync with deprovisioning enabled, an
// active enrollment of a OneRoster-managed user in a OneRoster-managed
// course that the roster no longer lists is dropped. Observer
// enrollments are skipped (they follow the child). The same circuit
// breaker as the user pass guards against a truncated roster.
func (s *OneRosterService) removeAbsentEnrollments(ctx context.Context, accountID uint, present map[onerosterEnrollmentKey]bool) (removed int, abortReason string, errs []string) {
	var managed []models.Enrollment
	if err := s.db.WithContext(ctx).
		Where("workflow_state = ? AND type <> ?", models.EnrollmentActive, "ObserverEnrollment").
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ? AND sis_course_id LIKE ?)", accountID, sisManagedPattern).
		Where("user_id IN (SELECT id FROM users WHERE account_id = ? AND sis_user_id LIKE ?)", accountID, sisManagedPattern).
		Find(&managed).Error; err != nil {
		return 0, "", []string{fmt.Sprintf("listing managed enrollments: %v", err)}
	}

	var absent []models.Enrollment
	for _, e := range managed {
		if !present[onerosterEnrollmentKey{e.UserID, e.CourseID, e.Type}] {
			absent = append(absent, e)
		}
	}

	switch {
	case len(present) == 0 && len(managed) > 0:
		return 0, fmt.Sprintf("refusing to remove enrollments against an empty enrollment roster (%d managed enrollments)", len(managed)), nil
	case len(absent) > deprovisionThreshold(len(managed)):
		return 0, fmt.Sprintf("refusing to remove %d of %d managed enrollments (threshold %d): the roster fetch may be incomplete",
			len(absent), len(managed), deprovisionThreshold(len(managed))), nil
	}

	for i := range absent {
		absent[i].WorkflowState = models.EnrollmentDeleted
		if err := s.enrollmentRepo.Update(ctx, &absent[i]); err != nil {
			errs = append(errs, fmt.Sprintf("failed to remove absent enrollment %d: %v", absent[i].ID, err))
			continue
		}
		removed++
	}
	return removed, "", errs
}

// ---- Role mapping ----

// mapOneRosterRole returns the Paper user role a newly provisioned
// OneRoster user starts with. It is always "user" (B074): a SIS
// "administrator" is a principal, counselor or district staffer, not a
// Paper tenant admin able to masquerade, edit SSO and run imports.
// Canvas likewise never grants account-admin from roster data — account
// admins are assigned explicitly. Course-level power for administrators
// comes from their enrollments (see mapOneRosterEnrollmentRole). The
// sync never rewrites an existing user's role, so a promotion or
// demotion made in Paper survives every sync.
func mapOneRosterRole(role string) string {
	_ = role
	return "user"
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
