package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/EduThemes/paper-lms/internal/auth/initialpassword"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type SISImportService struct {
	batchRepo      repository.SISBatchRepository
	errorRepo      repository.SISBatchErrorRepository
	userRepo       repository.UserRepository
	courseRepo      repository.CourseRepository
	sectionRepo    repository.SectionRepository
	enrollmentRepo repository.EnrollmentRepository
	db             *gorm.DB
	// sectionService (SetSectionService) applies xlists.csv rows.
	sectionService *SectionService
}

func NewSISImportService(
	batchRepo repository.SISBatchRepository,
	errorRepo repository.SISBatchErrorRepository,
	userRepo repository.UserRepository,
	courseRepo repository.CourseRepository,
	sectionRepo repository.SectionRepository,
	enrollmentRepo repository.EnrollmentRepository,
	db *gorm.DB,
) *SISImportService {
	return &SISImportService{
		batchRepo:      batchRepo,
		errorRepo:      errorRepo,
		userRepo:       userRepo,
		courseRepo:      courseRepo,
		sectionRepo:    sectionRepo,
		enrollmentRepo: enrollmentRepo,
		db:             db,
	}
}

func (s *SISImportService) CreateBatch(ctx context.Context, accountID uint) (*models.SISBatch, error) {
	batch := &models.SISBatch{
		AccountID:     accountID,
		WorkflowState: "created",
	}
	if err := s.batchRepo.Create(ctx, batch); err != nil {
		return nil, err
	}
	return batch, nil
}

// ErrSISBatchNotFound is returned when a batch does not exist or lives in
// another account than the caller is scoped to (indistinguishable on
// purpose: cross-tenant reads 404).
var ErrSISBatchNotFound = errors.New("sis batch not found")

// GetBatch loads a batch by id within accountID. accountID==0 is the
// internal escape hatch (no tenant filter) used by ProcessImport, which
// only ever receives a batch id it just created.
func (s *SISImportService) GetBatch(ctx context.Context, id, accountID uint) (*models.SISBatch, error) {
	batch, err := s.batchRepo.FindByID(ctx, id)
	if err != nil || batch == nil {
		return nil, ErrSISBatchNotFound
	}
	if accountID != 0 && batch.AccountID != accountID {
		return nil, ErrSISBatchNotFound
	}
	return batch, nil
}

func (s *SISImportService) ListBatches(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SISBatch], error) {
	return s.batchRepo.ListByAccountID(ctx, accountID, params)
}

// GetBatchErrors lists a batch's row errors after tying the batch to
// accountID (see GetBatch).
func (s *SISImportService) GetBatchErrors(ctx context.Context, batchID, accountID uint) ([]models.SISBatchError, error) {
	if _, err := s.GetBatch(ctx, batchID, accountID); err != nil {
		return nil, err
	}
	return s.errorRepo.ListByBatchID(ctx, batchID)
}

// ProcessImport runs one CSV against the batch. Every row is scoped to
// the batch's account: users are created in it, SIS-ID lookups only
// match rows in it, and courses can never be placed in another tenant.
func (s *SISImportService) ProcessImport(ctx context.Context, batchID uint, csvType string, reader io.Reader) error {
	batch, err := s.batchRepo.FindByID(ctx, batchID)
	if err != nil {
		return fmt.Errorf("batch not found: %w", err)
	}
	if batch.AccountID == 0 {
		return errors.New("sis batch has no account")
	}
	accountID := batch.AccountID

	// Update to importing state
	batch.WorkflowState = "importing"
	if err := s.batchRepo.Update(ctx, batch); err != nil {
		return err
	}

	csvReader := csv.NewReader(reader)

	var processErr error
	switch csvType {
	case "users":
		processErr = s.processUsersCSV(ctx, batchID, accountID, csvReader)
	case "courses":
		processErr = s.processCoursesCSV(ctx, batchID, accountID, csvReader)
	case "sections":
		processErr = s.processSectionsCSV(ctx, batchID, accountID, csvReader)
	case "enrollments":
		processErr = s.processEnrollmentsCSV(ctx, batchID, accountID, csvReader)
	case "xlists":
		processErr = s.processXlistsCSV(ctx, batchID, accountID, csvReader)
	default:
		processErr = fmt.Errorf("unknown import type: %s", csvType)
	}

	// Reload batch to get updated row counts
	batch, _ = s.batchRepo.FindByID(ctx, batchID)

	if processErr != nil {
		batch.WorkflowState = "failed"
		batch.Progress = 100
		s.batchRepo.Update(ctx, batch)
		return processErr
	}

	// Check if there were any errors recorded
	errors, _ := s.errorRepo.ListByBatchID(ctx, batchID)
	if len(errors) > 0 {
		batch.WorkflowState = "imported_with_messages"
	} else {
		batch.WorkflowState = "imported"
	}
	batch.Progress = 100
	s.batchRepo.Update(ctx, batch)

	return nil
}

func (s *SISImportService) processUsersCSV(ctx context.Context, batchID, accountID uint, reader *csv.Reader) error {
	// Read header row
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)

	batch, _ := s.batchRepo.FindByID(ctx, batchID)
	rowNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to read row: %v", err), "users.csv")
			rowNum++
			continue
		}

		rowNum++
		batch.TotalRows = rowNum - 1

		sisUserID := getColumn(record, colIndex, "user_id")
		loginID := getColumn(record, colIndex, "login_id")
		password := getColumn(record, colIndex, "password")
		firstName := getColumn(record, colIndex, "first_name")
		lastName := getColumn(record, colIndex, "last_name")
		email := getColumn(record, colIndex, "email")
		status := getColumn(record, colIndex, "status")

		if sisUserID == "" {
			s.recordError(ctx, batchID, rowNum, "user_id is required", "users.csv")
			continue
		}

		if status == "deleted" {
			batch.ProcessedRows++
			continue
		}

		// Try to find existing user by SIS user ID — in this tenant only.
		existingUser, findErr := s.findUserBySISID(ctx, sisUserID, accountID)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up user: %v", findErr), "users.csv")
			continue
		}

		if existingUser != nil {
			// Update existing user
			if name := strings.TrimSpace(firstName + " " + lastName); name != "" {
				existingUser.Name = name
			}
			if loginID != "" {
				existingUser.LoginID = loginID
			}
			if email != "" {
				existingUser.Email = email
			}
			if firstName != "" && lastName != "" {
				existingUser.SortableName = lastName + ", " + firstName
				existingUser.ShortName = firstName
			}
			if err := s.userRepo.Update(ctx, existingUser); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to update user: %v", err), "users.csv")
				continue
			}
		} else {
			// Create new user
			name := strings.TrimSpace(firstName + " " + lastName)
			sortableName := name
			if firstName != "" && lastName != "" {
				sortableName = lastName + ", " + firstName
			}

			// SIS user IDs are globally unique (idx_users_sis_user_id);
			// a collision with another tenant's user must not be
			// resolved by touching that user. Refuse the row instead.
			if s.sisIDTaken(ctx, "users", "sis_user_id", sisUserID) {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("user_id '%s' is already in use", sisUserID), "users.csv")
				continue
			}

			user := &models.User{
				AccountID:    accountID,
				Name:         name,
				SortableName: sortableName,
				ShortName:    firstName,
				LoginID:      loginID,
				Email:        email,
				SISUserID:    &sisUserID,
			}

			if password != "" {
				if err := user.HashPassword(password); err != nil {
					s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to hash password: %v", err), "users.csv")
					continue
				}
			} else {
				// No password supplied in the CSV — generate a
				// cryptographically random initial password. The plaintext
				// is irrecoverable; the user MUST go through the
				// password-reset flow (or use SSO) before they can log in.
				// Prior code stored bcrypt("changeme") for every such row,
				// which is the canonical default-credential vulnerability.
				initialPW, pwErr := initialpassword.GenerateInitialPassword()
				if pwErr != nil {
					s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to generate initial password: %v", pwErr), "users.csv")
					continue
				}
				if err := user.HashPassword(initialPW); err != nil {
					s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to hash default password: %v", err), "users.csv")
					continue
				}
				// Wave 1.6 follow-up: the random initial password
				// is irrecoverable, so flag the row so the
				// LoginPipeline gates session minting and forces a
				// real password before the user gets any session.
				// Only set when the CSV did not supply a password —
				// an admin who explicitly typed a password in the
				// import file is expected to communicate it out-of-
				// band and is not subject to the gate.
				user.RequiresPasswordReset = true
			}

			if err := s.userRepo.Create(ctx, user); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to create user: %v", err), "users.csv")
				continue
			}
		}

		batch.ProcessedRows++
		if batch.TotalRows > 0 {
			batch.Progress = (batch.ProcessedRows * 100) / batch.TotalRows
		}
		s.batchRepo.Update(ctx, batch)
	}

	batch.TotalRows = rowNum - 1
	s.batchRepo.Update(ctx, batch)
	return nil
}

func (s *SISImportService) processCoursesCSV(ctx context.Context, batchID, accountID uint, reader *csv.Reader) error {
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)

	batch, _ := s.batchRepo.FindByID(ctx, batchID)
	rowNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to read row: %v", err), "courses.csv")
			rowNum++
			continue
		}

		rowNum++
		batch.TotalRows = rowNum - 1

		sisCourseID := getColumn(record, colIndex, "course_id")
		shortName := getColumn(record, colIndex, "short_name")
		longName := getColumn(record, colIndex, "long_name")
		accountIDStr := getColumn(record, colIndex, "account_id")
		status := getColumn(record, colIndex, "status")

		if sisCourseID == "" {
			s.recordError(ctx, batchID, rowNum, "course_id is required", "courses.csv")
			continue
		}

		// Map status to workflow state
		workflowState := models.CourseAvailable
		if status == "active" {
			workflowState = models.CourseAvailable
		} else if status == "deleted" {
			workflowState = models.CourseDeleted
		}

		// Courses always land in the batch's account. Canvas's
		// courses.csv account_id names a (sub-)account by SIS id; Paper
		// tenants are single-level, so the column is accepted only when
		// it names the batch's own account (by sis_account_id or numeric
		// id). Anything else is refused — a CSV must never be able to
		// place a course in another tenant.
		if accountIDStr != "" && !s.accountRefMatches(ctx, accountIDStr, accountID) {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("account_id '%s' does not match the importing account", accountIDStr), "courses.csv")
			continue
		}

		// Try to find existing course by SIS course ID (this tenant only)
		existingCourse, findErr := s.findCourseBySISID(ctx, sisCourseID, accountID)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up course: %v", findErr), "courses.csv")
			continue
		}

		if existingCourse != nil {
			// Update existing course
			if longName != "" {
				existingCourse.Name = longName
			}
			if shortName != "" {
				existingCourse.CourseCode = shortName
			}
			existingCourse.WorkflowState = workflowState
			if err := s.courseRepo.Update(ctx, existingCourse); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to update course: %v", err), "courses.csv")
				continue
			}
		} else {
			if s.sisIDTaken(ctx, "courses", "sis_course_id", sisCourseID) {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course_id '%s' is already in use", sisCourseID), "courses.csv")
				continue
			}
			course := &models.Course{
				Name:          longName,
				CourseCode:    shortName,
				SISCourseID:   &sisCourseID,
				AccountID:     accountID,
				WorkflowState: workflowState,
			}
			if err := s.courseRepo.Create(ctx, course); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to create course: %v", err), "courses.csv")
				continue
			}
		}

		batch.ProcessedRows++
		if batch.TotalRows > 0 {
			batch.Progress = (batch.ProcessedRows * 100) / batch.TotalRows
		}
		s.batchRepo.Update(ctx, batch)
	}

	batch.TotalRows = rowNum - 1
	s.batchRepo.Update(ctx, batch)
	return nil
}

func (s *SISImportService) processSectionsCSV(ctx context.Context, batchID, accountID uint, reader *csv.Reader) error {
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)

	batch, _ := s.batchRepo.FindByID(ctx, batchID)
	rowNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to read row: %v", err), "sections.csv")
			rowNum++
			continue
		}

		rowNum++
		batch.TotalRows = rowNum - 1

		sisSectionID := getColumn(record, colIndex, "section_id")
		sisCourseID := getColumn(record, colIndex, "course_id")
		name := getColumn(record, colIndex, "name")
		status := getColumn(record, colIndex, "status")

		if sisSectionID == "" {
			s.recordError(ctx, batchID, rowNum, "section_id is required", "sections.csv")
			continue
		}

		if sisCourseID == "" {
			s.recordError(ctx, batchID, rowNum, "course_id is required", "sections.csv")
			continue
		}

		// Look up course by SIS course ID (this tenant only)
		course, findErr := s.findCourseBySISID(ctx, sisCourseID, accountID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course with sis_course_id '%s' not found", sisCourseID), "sections.csv")
			continue
		}

		workflowState := "active"
		if status == "deleted" {
			workflowState = "deleted"
		}

		// Try to find existing section by SIS section ID (this tenant only)
		existingSection, findErr := s.findSectionBySISID(ctx, sisSectionID, accountID)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up section: %v", findErr), "sections.csv")
			continue
		}

		startAt, endAt, dateErr := sisDates(record, colIndex)
		if dateErr != nil {
			s.recordError(ctx, batchID, rowNum, dateErr.Error(), "sections.csv")
			continue
		}

		if existingSection != nil {
			// Update existing section
			if name != "" {
				existingSection.Name = name
			}
			if existingSection.NonxlistCourseID != nil {
				// Cross-listed (xlists.csv / UI): the SIS course is the
				// home course; the section stays where it was cross-listed.
				existingSection.NonxlistCourseID = &course.ID
				if existingSection.CourseID == course.ID {
					existingSection.NonxlistCourseID = nil
				}
			} else {
				existingSection.CourseID = course.ID
			}
			existingSection.WorkflowState = workflowState
			applySISSectionDates(existingSection, startAt, endAt)
			if err := s.db.WithContext(ctx).Save(existingSection).Error; err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to update section: %v", err), "sections.csv")
				continue
			}
		} else {
			if s.sisIDTaken(ctx, "course_sections", "sis_section_id", sisSectionID) {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("section_id '%s' is already in use", sisSectionID), "sections.csv")
				continue
			}
			section := &models.CourseSection{
				CourseID:      course.ID,
				Name:          name,
				SISSectionID:  &sisSectionID,
				WorkflowState: workflowState,
			}
			applySISSectionDates(section, startAt, endAt)
			if err := s.sectionRepo.Create(ctx, section); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to create section: %v", err), "sections.csv")
				continue
			}
		}

		batch.ProcessedRows++
		if batch.TotalRows > 0 {
			batch.Progress = (batch.ProcessedRows * 100) / batch.TotalRows
		}
		s.batchRepo.Update(ctx, batch)
	}

	batch.TotalRows = rowNum - 1
	s.batchRepo.Update(ctx, batch)
	return nil
}

func (s *SISImportService) processEnrollmentsCSV(ctx context.Context, batchID, accountID uint, reader *csv.Reader) error {
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIndex := buildColumnIndex(header)

	batch, _ := s.batchRepo.FindByID(ctx, batchID)
	rowNum := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to read row: %v", err), "enrollments.csv")
			rowNum++
			continue
		}

		rowNum++
		batch.TotalRows = rowNum - 1

		sisCourseID := getColumn(record, colIndex, "course_id")
		sisUserID := getColumn(record, colIndex, "user_id")
		role := getColumn(record, colIndex, "role")
		sisSectionID := getColumn(record, colIndex, "section_id")
		status := getColumn(record, colIndex, "status")
		sisAssociatedUserID := getColumn(record, colIndex, "associated_user_id")

		if sisCourseID == "" {
			s.recordError(ctx, batchID, rowNum, "course_id is required", "enrollments.csv")
			continue
		}
		if sisUserID == "" {
			s.recordError(ctx, batchID, rowNum, "user_id is required", "enrollments.csv")
			continue
		}

		// Look up course by SIS course ID (this tenant only)
		course, findErr := s.findCourseBySISID(ctx, sisCourseID, accountID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course with sis_course_id '%s' not found", sisCourseID), "enrollments.csv")
			continue
		}

		// Look up user by SIS user ID (this tenant only)
		user, findErr := s.findUserBySISID(ctx, sisUserID, accountID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("user with sis_user_id '%s' not found", sisUserID), "enrollments.csv")
			continue
		}

		// Determine workflow state from status
		var workflowState models.EnrollmentWorkflow
		switch strings.ToLower(status) {
		case "", "active":
			workflowState = models.EnrollmentActive
		case "deleted":
			workflowState = models.EnrollmentDeleted
		case "inactive":
			workflowState = models.EnrollmentInactive
		case "completed", "concluded":
			workflowState = models.EnrollmentCompleted
		default:
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("invalid status '%s' (expected active, deleted, inactive or completed)", status), "enrollments.csv")
			continue
		}

		// Map role to enrollment type. Unknown roles are refused rather
		// than stored verbatim: a Type outside the five base types fails
		// every course-role check while still passing "is enrolled".
		enrollmentType, roleName, ok := s.mapSISEnrollmentRole(ctx, role, accountID)
		if !ok {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("invalid role '%s' (expected student, teacher, ta, observer, designer or a custom role name)", role), "enrollments.csv")
			continue
		}

		startAt, endAt, dateErr := sisDates(record, colIndex)
		if dateErr != nil {
			s.recordError(ctx, batchID, rowNum, dateErr.Error(), "enrollments.csv")
			continue
		}
		limitSection := strings.EqualFold(getColumn(record, colIndex, "limit_section_privileges"), "true")

		enrollment := &models.Enrollment{
			UserID:                         user.ID,
			CourseID:                       course.ID,
			Type:                           enrollmentType,
			Role:                           roleName,
			WorkflowState:                  workflowState,
			StartAt:                        startAt,
			EndAt:                          endAt,
			LimitPrivilegesToCourseSection: limitSection,
		}

		// Optionally look up section by SIS section ID. The section must
		// belong to the enrollment's course.
		if sisSectionID != "" {
			section, sectionErr := s.findSectionBySISID(ctx, sisSectionID, accountID)
			if sectionErr != nil || section.CourseID != course.ID {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("section with sis_section_id '%s' not found in course '%s'", sisSectionID, sisCourseID), "enrollments.csv")
				continue
			}
			enrollment.CourseSectionID = &section.ID
		}

		// Observers may name the observed student (Canvas's
		// associated_user_id column, a SIS user id).
		if sisAssociatedUserID != "" && enrollmentType == "ObserverEnrollment" {
			observee, obsErr := s.findUserBySISID(ctx, sisAssociatedUserID, accountID)
			if obsErr != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("associated user with sis_user_id '%s' not found", sisAssociatedUserID), "enrollments.csv")
				continue
			}
			enrollment.AssociatedUserID = &observee.ID
		}

		// Deduplicate on re-import. Canvas identifies an SIS enrollment
		// by (user, course, type, associated user) in ANY state, so an
		// inactive/completed/deleted row is updated in place rather
		// than inserted again on every nightly run.
		existingEnrollment, lookupErr := s.findSISEnrollment(ctx, enrollment)
		if lookupErr != nil && lookupErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up enrollment: %v", lookupErr), "enrollments.csv")
			continue
		}
		if existingEnrollment != nil {
			existingEnrollment.WorkflowState = workflowState
			existingEnrollment.Role = roleName
			if enrollment.CourseSectionID != nil {
				existingEnrollment.CourseSectionID = enrollment.CourseSectionID
			}
			if _, ok := colIndex["start_date"]; ok {
				existingEnrollment.StartAt = startAt
			}
			if _, ok := colIndex["end_date"]; ok {
				existingEnrollment.EndAt = endAt
			}
			if _, ok := colIndex["limit_section_privileges"]; ok {
				existingEnrollment.LimitPrivilegesToCourseSection = limitSection
			}
			if err := s.enrollmentRepo.Update(ctx, existingEnrollment); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to update enrollment: %v", err), "enrollments.csv")
				continue
			}
		} else {
			if err := s.enrollmentRepo.Create(ctx, enrollment); err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to create enrollment: %v", err), "enrollments.csv")
				continue
			}
		}

		batch.ProcessedRows++
		if batch.TotalRows > 0 {
			batch.Progress = (batch.ProcessedRows * 100) / batch.TotalRows
		}
		s.batchRepo.Update(ctx, batch)
	}

	batch.TotalRows = rowNum - 1
	s.batchRepo.Update(ctx, batch)
	return nil
}

// sisBaseRoleTypes maps the Canvas enrollments.csv base role keywords
// (and the full type names) onto Paper's enrollment Type.
var sisBaseRoleTypes = map[string]string{
	"student":            "StudentEnrollment",
	"teacher":            "TeacherEnrollment",
	"ta":                 "TaEnrollment",
	"observer":           "ObserverEnrollment",
	"designer":           "DesignerEnrollment",
	"studentenrollment":  "StudentEnrollment",
	"teacherenrollment":  "TeacherEnrollment",
	"taenrollment":       "TaEnrollment",
	"observerenrollment": "ObserverEnrollment",
	"designerenrollment": "DesignerEnrollment",
}

// mapSISEnrollmentRole resolves an enrollments.csv role value into
// (Type, Role). Base roles match case-insensitively; anything else must
// name an active custom role in the tenant whose base type is a course
// role, in which case Type is the base type and Role keeps the custom
// role's name (Canvas semantics). ok=false means the value is invalid.
func (s *SISImportService) mapSISEnrollmentRole(ctx context.Context, role string, accountID uint) (string, string, bool) {
	key := strings.ToLower(strings.TrimSpace(role))
	if key == "" {
		return "", "", false
	}
	if t, ok := sisBaseRoleTypes[key]; ok {
		return t, t, true
	}
	if s.db == nil {
		return "", "", false
	}
	var custom models.CustomRole
	err := s.db.WithContext(ctx).
		Where("account_id = ? AND LOWER(name) = ? AND workflow_state = ?", accountID, key, "active").
		First(&custom).Error
	if err != nil {
		return "", "", false
	}
	t, ok := sisBaseRoleTypes[strings.ToLower(custom.BaseRoleType)]
	if !ok {
		// e.g. an "admin"-based custom role is not a course enrollment.
		return "", "", false
	}
	return t, custom.Name, true
}

// findSISEnrollment returns the enrollment an enrollments.csv row
// refers to, in any workflow state, preferring a non-deleted row.
func (s *SISImportService) findSISEnrollment(ctx context.Context, e *models.Enrollment) (*models.Enrollment, error) {
	q := s.db.WithContext(ctx).
		Where("user_id = ? AND course_id = ? AND type = ?", e.UserID, e.CourseID, e.Type)
	if e.AssociatedUserID != nil {
		q = q.Where("associated_user_id = ?", *e.AssociatedUserID)
	} else {
		q = q.Where("associated_user_id IS NULL")
	}
	var existing models.Enrollment
	if err := q.Order("(workflow_state = 'deleted') ASC, id ASC").First(&existing).Error; err != nil {
		return nil, err
	}
	return &existing, nil
}

// Export methods

func (s *SISImportService) ExportUsersCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&users).Error; err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	writer.Write([]string{"user_id", "login_id", "first_name", "last_name", "email", "status"})

	for _, u := range users {
		sisID := ""
		if u.SISUserID != nil {
			sisID = *u.SISUserID
		}

		firstName, lastName := splitName(u.Name)

		_ = writeCSVRow(writer, []string{
			sisID,
			u.LoginID,
			firstName,
			lastName,
			u.Email,
			"active",
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportCoursesCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var courses []models.Course
	if err := s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses).Error; err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	writer.Write([]string{"course_id", "short_name", "long_name", "account_id", "status"})

	for _, c := range courses {
		sisID := ""
		if c.SISCourseID != nil {
			sisID = *c.SISCourseID
		}

		status := "active"
		if c.WorkflowState == "deleted" {
			status = "deleted"
		}

		_ = writeCSVRow(writer, []string{
			sisID,
			c.CourseCode,
			c.Name,
			strconv.FormatUint(uint64(c.AccountID), 10),
			status,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportSectionsCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var sections []models.CourseSection
	if err := s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&sections).Error; err != nil {
		return nil, err
	}

	// Build a map of course ID to SIS course ID for lookups (tenant-scoped)
	var courses []models.Course
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses)
	courseMap := make(map[uint]string)
	for _, c := range courses {
		if c.SISCourseID != nil {
			courseMap[c.ID] = *c.SISCourseID
		}
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	writer.Write([]string{"section_id", "course_id", "name", "status"})

	for _, sec := range sections {
		sisSectionID := ""
		if sec.SISSectionID != nil {
			sisSectionID = *sec.SISSectionID
		}

		sisCourseID := courseMap[sec.CourseID]

		status := "active"
		if sec.WorkflowState == "deleted" {
			status = "deleted"
		}

		_ = writeCSVRow(writer, []string{
			sisSectionID,
			sisCourseID,
			sec.Name,
			status,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *SISImportService) ExportEnrollmentsCSV(ctx context.Context, accountID uint) ([]byte, error) {
	var enrollments []models.Enrollment
	if err := s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&enrollments).Error; err != nil {
		return nil, err
	}

	// Build lookup maps for SIS IDs (tenant-scoped)
	var courses []models.Course
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&courses)
	courseMap := make(map[uint]string)
	for _, c := range courses {
		if c.SISCourseID != nil {
			courseMap[c.ID] = *c.SISCourseID
		}
	}

	var users []models.User
	s.db.WithContext(ctx).Where("account_id = ?", accountID).Find(&users)
	userMap := make(map[uint]string)
	for _, u := range users {
		if u.SISUserID != nil {
			userMap[u.ID] = *u.SISUserID
		}
	}

	var sections []models.CourseSection
	s.db.WithContext(ctx).
		Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID).
		Find(&sections)
	sectionMap := make(map[uint]string)
	for _, sec := range sections {
		if sec.SISSectionID != nil {
			sectionMap[sec.ID] = *sec.SISSectionID
		}
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	writer.Write([]string{"course_id", "user_id", "role", "section_id", "status"})

	for _, e := range enrollments {
		sisCourseID := courseMap[e.CourseID]
		sisUserID := userMap[e.UserID]

		sisSectionID := ""
		if e.CourseSectionID != nil {
			sisSectionID = sectionMap[*e.CourseSectionID]
		}

		status := string(e.WorkflowState)

		_ = writeCSVRow(writer, []string{
			sisCourseID,
			sisUserID,
			e.Role,
			sisSectionID,
			status,
		})
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Helper methods for SIS ID lookups

// Every SIS-ID lookup is tenant-scoped: a CSV uploaded by one tenant
// must never resolve (and then overwrite) another tenant's row.

func (s *SISImportService) findUserBySISID(ctx context.Context, sisID string, accountID uint) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("sis_user_id = ? AND account_id = ?", sisID, accountID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SISImportService) findCourseBySISID(ctx context.Context, sisID string, accountID uint) (*models.Course, error) {
	var course models.Course
	if err := s.db.WithContext(ctx).Where("sis_course_id = ? AND account_id = ?", sisID, accountID).First(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

func (s *SISImportService) findSectionBySISID(ctx context.Context, sisID string, accountID uint) (*models.CourseSection, error) {
	var section models.CourseSection
	if err := s.db.WithContext(ctx).
		Where("sis_section_id = ? AND course_id IN (SELECT id FROM courses WHERE account_id = ?)", sisID, accountID).
		First(&section).Error; err != nil {
		return nil, err
	}
	return &section, nil
}

// sisIDTaken reports whether a SIS id is already used anywhere (the
// sis_*_id unique indexes are deployment-wide). Called only after the
// tenant-scoped lookup missed, so a hit means another tenant owns it.
// table/column are compile-time constants, never user input.
func (s *SISImportService) sisIDTaken(ctx context.Context, table, column, sisID string) bool {
	var n int64
	if err := s.db.WithContext(ctx).Table(table).Where(column+" = ?", sisID).Count(&n).Error; err != nil {
		return false
	}
	return n > 0
}

// accountRefMatches reports whether a courses.csv account_id value names
// accountID, either by its sis_account_id or by its numeric id.
func (s *SISImportService) accountRefMatches(ctx context.Context, ref string, accountID uint) bool {
	var acct models.Account
	if err := s.db.WithContext(ctx).Where("id = ?", accountID).First(&acct).Error; err != nil {
		return false
	}
	if acct.SISAccountID != nil && *acct.SISAccountID == ref {
		return true
	}
	if parsed, err := strconv.ParseUint(ref, 10, 64); err == nil && uint(parsed) == accountID {
		return true
	}
	return false
}

func (s *SISImportService) recordError(ctx context.Context, batchID uint, row int, message string, file string) {
	batchError := &models.SISBatchError{
		SISBatchID: batchID,
		Row:        row,
		Message:    message,
		File:       file,
	}
	s.errorRepo.Create(ctx, batchError)
}

// Utility functions

func buildColumnIndex(header []string) map[string]int {
	index := make(map[string]int)
	for i, col := range header {
		index[strings.TrimSpace(strings.ToLower(col))] = i
	}
	return index
}

func getColumn(record []string, colIndex map[string]int, name string) string {
	if idx, ok := colIndex[name]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}

func splitName(name string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return name, ""
}
