package service

import (
	"bytes"
	"context"
	"encoding/csv"
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

func (s *SISImportService) GetBatch(ctx context.Context, id uint) (*models.SISBatch, error) {
	return s.batchRepo.FindByID(ctx, id)
}

func (s *SISImportService) ListBatches(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SISBatch], error) {
	return s.batchRepo.ListByAccountID(ctx, accountID, params)
}

func (s *SISImportService) GetBatchErrors(ctx context.Context, batchID uint) ([]models.SISBatchError, error) {
	return s.errorRepo.ListByBatchID(ctx, batchID)
}

func (s *SISImportService) ProcessImport(ctx context.Context, batchID uint, csvType string, reader io.Reader) error {
	batch, err := s.batchRepo.FindByID(ctx, batchID)
	if err != nil {
		return fmt.Errorf("batch not found: %w", err)
	}

	// Update to importing state
	batch.WorkflowState = "importing"
	if err := s.batchRepo.Update(ctx, batch); err != nil {
		return err
	}

	csvReader := csv.NewReader(reader)

	var processErr error
	switch csvType {
	case "users":
		processErr = s.processUsersCSV(ctx, batchID, csvReader)
	case "courses":
		processErr = s.processCoursesCSV(ctx, batchID, csvReader)
	case "sections":
		processErr = s.processSectionsCSV(ctx, batchID, csvReader)
	case "enrollments":
		processErr = s.processEnrollmentsCSV(ctx, batchID, csvReader)
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

func (s *SISImportService) processUsersCSV(ctx context.Context, batchID uint, reader *csv.Reader) error {
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

		// Try to find existing user by SIS user ID
		existingUser, findErr := s.findUserBySISID(ctx, sisUserID)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up user: %v", findErr), "users.csv")
			continue
		}

		if existingUser != nil {
			// Update existing user
			existingUser.Name = strings.TrimSpace(firstName + " " + lastName)
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

			user := &models.User{
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

func (s *SISImportService) processCoursesCSV(ctx context.Context, batchID uint, reader *csv.Reader) error {
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
		workflowState := "available"
		if status == "active" {
			workflowState = "available"
		} else if status == "deleted" {
			workflowState = "deleted"
		}

		accountID := uint(1)
		if accountIDStr != "" {
			if parsed, parseErr := strconv.ParseUint(accountIDStr, 10, 64); parseErr == nil {
				accountID = uint(parsed)
			}
		}

		// Try to find existing course by SIS course ID
		existingCourse, findErr := s.findCourseBySISID(ctx, sisCourseID)
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

func (s *SISImportService) processSectionsCSV(ctx context.Context, batchID uint, reader *csv.Reader) error {
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

		// Look up course by SIS course ID
		course, findErr := s.findCourseBySISID(ctx, sisCourseID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course with sis_course_id '%s' not found", sisCourseID), "sections.csv")
			continue
		}

		workflowState := "active"
		if status == "deleted" {
			workflowState = "deleted"
		}

		// Try to find existing section by SIS section ID
		existingSection, findErr := s.findSectionBySISID(ctx, sisSectionID)
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("error looking up section: %v", findErr), "sections.csv")
			continue
		}

		if existingSection != nil {
			// Update existing section
			if name != "" {
				existingSection.Name = name
			}
			existingSection.CourseID = course.ID
			existingSection.WorkflowState = workflowState
			if err := s.db.WithContext(ctx).Save(existingSection).Error; err != nil {
				s.recordError(ctx, batchID, rowNum, fmt.Sprintf("failed to update section: %v", err), "sections.csv")
				continue
			}
		} else {
			section := &models.CourseSection{
				CourseID:      course.ID,
				Name:          name,
				SISSectionID:  &sisSectionID,
				WorkflowState: workflowState,
			}
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

func (s *SISImportService) processEnrollmentsCSV(ctx context.Context, batchID uint, reader *csv.Reader) error {
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

		if sisCourseID == "" {
			s.recordError(ctx, batchID, rowNum, "course_id is required", "enrollments.csv")
			continue
		}
		if sisUserID == "" {
			s.recordError(ctx, batchID, rowNum, "user_id is required", "enrollments.csv")
			continue
		}

		// Look up course by SIS course ID
		course, findErr := s.findCourseBySISID(ctx, sisCourseID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("course with sis_course_id '%s' not found", sisCourseID), "enrollments.csv")
			continue
		}

		// Look up user by SIS user ID
		user, findErr := s.findUserBySISID(ctx, sisUserID)
		if findErr != nil {
			s.recordError(ctx, batchID, rowNum, fmt.Sprintf("user with sis_user_id '%s' not found", sisUserID), "enrollments.csv")
			continue
		}

		// Determine workflow state from status
		workflowState := "active"
		if status == "deleted" || status == "inactive" {
			workflowState = status
		} else if status == "completed" {
			workflowState = "completed"
		}

		// Map role to enrollment type
		enrollmentType := role
		if role == "student" {
			enrollmentType = "StudentEnrollment"
		} else if role == "teacher" {
			enrollmentType = "TeacherEnrollment"
		} else if role == "ta" {
			enrollmentType = "TaEnrollment"
		} else if role == "observer" {
			enrollmentType = "ObserverEnrollment"
		} else if role == "designer" {
			enrollmentType = "DesignerEnrollment"
		}

		enrollment := &models.Enrollment{
			UserID:        user.ID,
			CourseID:      course.ID,
			Type:          enrollmentType,
			Role:          enrollmentType,
			WorkflowState: workflowState,
		}

		// Optionally look up section by SIS section ID
		if sisSectionID != "" {
			section, sectionErr := s.findSectionBySISID(ctx, sisSectionID)
			if sectionErr == nil {
				enrollment.CourseSectionID = &section.ID
			}
		}

		// Check for existing enrollment (deduplication on re-import)
		existingEnrollment, _ := s.enrollmentRepo.FindByUserAndCourse(ctx, user.ID, course.ID)
		if existingEnrollment != nil && existingEnrollment.Type == enrollmentType {
			// Update existing enrollment instead of creating a duplicate
			existingEnrollment.WorkflowState = workflowState
			if enrollment.CourseSectionID != nil {
				existingEnrollment.CourseSectionID = enrollment.CourseSectionID
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

// Export methods

func (s *SISImportService) ExportUsersCSV(ctx context.Context) ([]byte, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Find(&users).Error; err != nil {
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

		writer.Write([]string{
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

func (s *SISImportService) ExportCoursesCSV(ctx context.Context) ([]byte, error) {
	var courses []models.Course
	if err := s.db.WithContext(ctx).Find(&courses).Error; err != nil {
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

		writer.Write([]string{
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

func (s *SISImportService) ExportSectionsCSV(ctx context.Context) ([]byte, error) {
	var sections []models.CourseSection
	if err := s.db.WithContext(ctx).Find(&sections).Error; err != nil {
		return nil, err
	}

	// Build a map of course ID to SIS course ID for lookups
	var courses []models.Course
	s.db.WithContext(ctx).Find(&courses)
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

		writer.Write([]string{
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

func (s *SISImportService) ExportEnrollmentsCSV(ctx context.Context) ([]byte, error) {
	var enrollments []models.Enrollment
	if err := s.db.WithContext(ctx).Find(&enrollments).Error; err != nil {
		return nil, err
	}

	// Build lookup maps for SIS IDs
	var courses []models.Course
	s.db.WithContext(ctx).Find(&courses)
	courseMap := make(map[uint]string)
	for _, c := range courses {
		if c.SISCourseID != nil {
			courseMap[c.ID] = *c.SISCourseID
		}
	}

	var users []models.User
	s.db.WithContext(ctx).Find(&users)
	userMap := make(map[uint]string)
	for _, u := range users {
		if u.SISUserID != nil {
			userMap[u.ID] = *u.SISUserID
		}
	}

	var sections []models.CourseSection
	s.db.WithContext(ctx).Find(&sections)
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

		status := e.WorkflowState

		writer.Write([]string{
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

func (s *SISImportService) findUserBySISID(ctx context.Context, sisID string) (*models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("sis_user_id = ?", sisID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *SISImportService) findCourseBySISID(ctx context.Context, sisID string) (*models.Course, error) {
	var course models.Course
	if err := s.db.WithContext(ctx).Where("sis_course_id = ?", sisID).First(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

func (s *SISImportService) findSectionBySISID(ctx context.Context, sisID string) (*models.CourseSection, error) {
	var section models.CourseSection
	if err := s.db.WithContext(ctx).Where("sis_section_id = ?", sisID).First(&section).Error; err != nil {
		return nil, err
	}
	return &section, nil
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
