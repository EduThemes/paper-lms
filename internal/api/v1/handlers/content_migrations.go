package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ContentMigrationHandler struct {
	migrationService *service.ContentMigrationService

	// Course copy / worker wiring (SetCourseCopy). Without it course_copy
	// rows are recorded but never run.
	runner      *service.ContentMigrationRunner
	copier      *service.CourseCopyService
	courses     repository.CourseRepository
	enrollments repository.EnrollmentRepository
	storagePath string
}

func NewContentMigrationHandler(migrationService *service.ContentMigrationService) *ContentMigrationHandler {
	return &ContentMigrationHandler{migrationService: migrationService}
}

// ContentMigrationDeps wires the course-copy worker and the source-course
// authorization into the handler.
type ContentMigrationDeps struct {
	Runner      *service.ContentMigrationRunner
	Copier      *service.CourseCopyService
	Courses     repository.CourseRepository
	Enrollments repository.EnrollmentRepository
	// StoragePath is where uploaded packages are kept until imported.
	StoragePath string
}

// SetCourseCopy wires the worker and course-copy dependencies.
func (h *ContentMigrationHandler) SetCourseCopy(d ContentMigrationDeps) {
	h.runner = d.Runner
	h.copier = d.Copier
	h.courses = d.Courses
	h.enrollments = d.Enrollments
	h.storagePath = d.StoragePath
}

func contentMigrationToJSON(m *models.ContentMigration) fiber.Map {
	attachment := ""
	if m.Attachment != "" {
		// Never echo the server-side path.
		attachment = filepath.Base(m.Attachment)
	}
	results := m.Results.Data()
	return fiber.Map{
		"id":                     m.ID,
		"course_id":              m.CourseID,
		"user_id":                m.UserID,
		"migration_type":         m.MigrationType,
		"source_course_id":       m.SourceCourseID,
		"workflow_state":         m.WorkflowState,
		"progress":               m.Progress,
		"migration_settings":     m.MigrationSettings.Data(),
		"started_at":             m.StartedAt,
		"finished_at":            m.FinishedAt,
		"error_message":          m.ErrorMessage,
		"attachment":             attachment,
		"results":                results,
		"migration_issues_count": len(results.Warnings),
		"created_at":             m.CreatedAt,
		"updated_at":             m.UpdatedAt,
	}
}

// parseSettings decodes a raw migration_settings payload from the API.
// Accepts either a JSON object (preferred) or a legacy JSON-encoded string;
// any malformed input is preserved on LegacyString so the row is never lost.
func parseSettings(raw json.RawMessage) models.MigrationSettings {
	var s models.MigrationSettings
	if len(raw) == 0 || string(raw) == "null" {
		return s
	}
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Try unwrapping a JSON-encoded string ("{\"foo\":1}" or arbitrary text).
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		if asString == "" {
			return s
		}
		if err := json.Unmarshal([]byte(asString), &s); err == nil {
			return s
		}
		s.LegacyString = asString
		return s
	}
	s.LegacyString = string(raw)
	return s
}

func (h *ContentMigrationHandler) ListMigrations(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.migrationService.ListMigrations(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch content migrations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	migrations := make([]fiber.Map, len(result.Items))
	for i, m := range result.Items {
		migrations[i] = contentMigrationToJSON(&m)
	}

	return c.JSON(migrations)
}

// copySourceRoles are the enrollments that may copy a course's content
// elsewhere (Canvas: manage content / read_as_admin).
var copySourceRoles = map[string]bool{
	"TeacherEnrollment":  true,
	"TaEnrollment":       true,
	"DesignerEnrollment": true,
}

// authorizeCopySource reports whether the caller may copy content out of
// sourceCourseID: it must be a live course of the caller's tenant, and
// the caller an admin or an active teacher/TA/designer of it. Any miss is
// reported the same way so the endpoint cannot probe course ids.
func (h *ContentMigrationHandler) authorizeCopySource(c *fiber.Ctx, sourceCourseID uint) (*models.Course, bool) {
	if h.courses == nil || sourceCourseID == 0 {
		return nil, false
	}
	account := callerAccountID(c)
	course, err := h.courses.FindByID(c.Context(), sourceCourseID, account)
	if err != nil || course == nil || course.AccountID != account || course.WorkflowState == models.CourseDeleted {
		return nil, false
	}
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return course, true
	}
	if h.enrollments == nil {
		return nil, false
	}
	userID, _ := c.Locals("user_id").(uint)
	e, err := h.enrollments.FindByUserAndCourse(c.Context(), userID, sourceCourseID, account)
	if err != nil || e == nil || e.WorkflowState != "active" || !copySourceRoles[e.Type] {
		return nil, false
	}
	return course, true
}

// migrationInput is the POST /content_migrations body. Canvas clients
// send the options at the top level (settings, date_shift_options,
// select); Paper's UI nests them under content_migration. Both work.
type migrationInput struct {
	MigrationType     string            `json:"migration_type"`
	SourceCourseID    *uint             `json:"source_course_id"`
	Settings          json.RawMessage   `json:"settings"`
	MigrationSettings json.RawMessage   `json:"migration_settings"`
	DateShiftOptions  json.RawMessage   `json:"date_shift_options"`
	Select            map[string][]uint `json:"select"`
}

func (in migrationInput) merge(other migrationInput) migrationInput {
	if in.MigrationType == "" {
		in.MigrationType = other.MigrationType
	}
	if in.SourceCourseID == nil {
		in.SourceCourseID = other.SourceCourseID
	}
	if len(in.Settings) == 0 {
		in.Settings = other.Settings
	}
	if len(in.MigrationSettings) == 0 {
		in.MigrationSettings = other.MigrationSettings
	}
	if len(in.DateShiftOptions) == 0 {
		in.DateShiftOptions = other.DateShiftOptions
	}
	if len(in.Select) == 0 {
		in.Select = other.Select
	}
	return in
}

func (h *ContentMigrationHandler) CreateMigration(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var body struct {
		migrationInput
		ContentMigration migrationInput `json:"content_migration"`
	}
	in := migrationInput{}
	if strings.HasPrefix(string(c.Request().Header.ContentType()), fiber.MIMEMultipartForm) {
		// Multipart upload: the options arrive as form fields.
		in.MigrationType = c.FormValue("migration_type")
		if raw := c.FormValue("settings"); raw != "" {
			in.Settings = json.RawMessage(raw)
		}
		if raw := c.FormValue("date_shift_options"); raw != "" {
			in.DateShiftOptions = json.RawMessage(raw)
		}
	} else if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return responses.BadRequest(c, "Invalid input")
		}
		in = body.ContentMigration.merge(body.migrationInput)
	}

	rawSettings := in.Settings
	if len(rawSettings) == 0 {
		rawSettings = in.MigrationSettings
	}
	settings := parseSettings(rawSettings)
	if len(in.DateShiftOptions) > 0 && string(in.DateShiftOptions) != "null" {
		var dso models.DateShiftOptions
		if err := json.Unmarshal(in.DateShiftOptions, &dso); err != nil {
			return responses.BadRequest(c, "Invalid date_shift_options: "+err.Error())
		}
		settings.DateShiftOptions = &dso
	}
	if len(in.Select) > 0 {
		settings.Select = in.Select
	}
	sourceCourseID := in.SourceCourseID
	if sourceCourseID == nil && settings.SourceCourseID != nil {
		sourceCourseID = settings.SourceCourseID
	}

	userID, _ := c.Locals("user_id").(uint)
	migration := &models.ContentMigration{
		CourseID:       uint(courseID),
		UserID:         userID,
		MigrationType:  in.MigrationType,
		SourceCourseID: sourceCourseID,
	}

	if in.MigrationType == "course_copy" {
		if sourceCourseID == nil || *sourceCourseID == 0 {
			return responses.BadRequest(c, "source_course_id is required for course_copy")
		}
		if *sourceCourseID == uint(courseID) {
			return responses.BadRequest(c, "A course cannot be copied into itself")
		}
		if _, ok := h.authorizeCopySource(c, *sourceCourseID); !ok {
			return responses.NotFound(c, "source course")
		}
		settings.SourceCourseID = sourceCourseID
	}

	// Package upload: kept under the storage root with a generated name.
	if file, fileErr := c.FormFile("attachment"); fileErr == nil && file != nil {
		ext := strings.ToLower(filepath.Ext(file.Filename))
		if ext != ".imscc" && ext != ".zip" {
			return responses.BadRequest(c, "Only .imscc and .zip packages can be imported")
		}
		root := h.storagePath
		if root == "" {
			root = "uploads"
		}
		dir := filepath.Join(root, "content_migrations", strconv.Itoa(courseID))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return responses.InternalError(c, "Could not save uploaded file")
		}
		path := filepath.Join(dir, uuid.New().String()+ext)
		if err := c.SaveFile(file, path); err != nil {
			return responses.InternalError(c, "Could not save uploaded file")
		}
		migration.Attachment = path
	}

	migration.MigrationSettings = datatypes.NewJSONType(settings)
	runnable := h.runner != nil && h.runner.Runnable(migration)
	if runnable {
		migration.WorkflowState = models.ContentMigrationQueued
	}

	if err := h.migrationService.CreateMigration(c.Context(), migration); err != nil {
		if migration.Attachment != "" {
			_ = os.Remove(migration.Attachment)
		}
		return responses.BadRequest(c, err.Error())
	}
	if runnable {
		h.runner.Enqueue(migration.ID)
	}

	return c.Status(fiber.StatusCreated).JSON(contentMigrationToJSON(migration))
}

// SelectableContent handles
// GET /courses/:course_id/content_migrations/selectable?source_course_id=N
// — the source course's copyable content for the selective copy dialog
// (Canvas's selective_data). The caller must be allowed to copy from the
// source course.
func (h *ContentMigrationHandler) SelectableContent(c *fiber.Ctx) error {
	if h.copier == nil {
		return responses.NotFound(c, "course copy")
	}
	sourceID, err := strconv.ParseUint(c.Query("source_course_id"), 10, 64)
	if err != nil || sourceID == 0 {
		return responses.BadRequest(c, "source_course_id is required")
	}
	if _, ok := h.authorizeCopySource(c, uint(sourceID)); !ok {
		return responses.NotFound(c, "source course")
	}
	items, err := h.copier.SelectableContent(c.Context(), uint(sourceID))
	if err != nil {
		return responses.NotFound(c, "source course")
	}
	return c.JSON(items)
}

// CopySources handles GET /courses/:course_id/content_migrations/copy_sources?search=
// — the courses the caller may copy content from (other than this one):
// courses they teach (TA/designer included) or, for admins, any course of
// their account. At most 100 results.
func (h *ContentMigrationHandler) CopySources(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	if h.courses == nil {
		return c.JSON([]fiber.Map{})
	}
	account := callerAccountID(c)
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))
	matches := func(co *models.Course) bool {
		if co.ID == uint(courseID) || co.WorkflowState == models.CourseDeleted || co.AccountID != account {
			return false
		}
		if search == "" {
			return true
		}
		return strings.Contains(strings.ToLower(co.Name), search) || strings.Contains(strings.ToLower(co.CourseCode), search)
	}

	var courses []models.Course
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		res, err := h.courses.List(c.Context(), account, repository.PaginationParams{Page: 1, PerPage: 1000})
		if err != nil {
			return responses.InternalError(c, "Could not list courses")
		}
		for i := range res.Items {
			if matches(&res.Items[i]) {
				courses = append(courses, res.Items[i])
			}
		}
	} else if h.enrollments != nil {
		userID, _ := c.Locals("user_id").(uint)
		enrollments, err := h.enrollments.ListByUserID(c.Context(), userID, account)
		if err != nil {
			return responses.InternalError(c, "Could not list courses")
		}
		seen := map[uint]bool{}
		for _, e := range enrollments {
			if seen[e.CourseID] || e.WorkflowState != "active" || !copySourceRoles[e.Type] {
				continue
			}
			seen[e.CourseID] = true
			co, err := h.courses.FindByID(c.Context(), e.CourseID, account)
			if err == nil && co != nil && matches(co) {
				courses = append(courses, *co)
			}
		}
	}
	sort.SliceStable(courses, func(i, j int) bool { return courses[i].ID > courses[j].ID })
	if len(courses) > 100 {
		courses = courses[:100]
	}
	out := make([]fiber.Map, len(courses))
	for i, co := range courses {
		out[i] = fiber.Map{
			"id":          co.ID,
			"name":        co.Name,
			"course_code": co.CourseCode,
			"start_at":    co.StartAt,
			"end_at":      co.EndAt,
			"label":       fmt.Sprintf("%s (%s)", co.Name, co.CourseCode),
		}
	}
	return c.JSON(out)
}

// requireMigrationInCourse loads :id and verifies it belongs to the
// URL's :course_id (F-013 parent-tie). The instructor middleware has
// already tied :course_id to the caller and their tenant, so the course
// tie is also the tenant tie. Writes a 404 and returns wrote=true on any
// miss — the caller must `return nil`.
//
// B142: the repo's FindByID is keyed by id alone, so pre-fix an
// instructor of course A could read (error text, attachment path,
// settings) or rewrite any course's migration in any tenant.
func (h *ContentMigrationHandler) requireMigrationInCourse(c *fiber.Ctx) (*models.ContentMigration, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	migrationID, err := c.ParamsInt("id")
	if err != nil || migrationID <= 0 {
		_ = responses.BadRequest(c, "Invalid content migration ID")
		return nil, true
	}
	migration, err := h.migrationService.GetMigration(c.Context(), uint(migrationID))
	if err != nil || migration == nil || migration.CourseID != uint(courseID) {
		_ = responses.NotFound(c, "content migration")
		return nil, true
	}
	return migration, false
}

// validContentMigrationStates is the closed set of workflow_state values
// a PUT may write. queued/running belong to the worker: writing them by
// hand would re-run (or wedge) a migration.
var validContentMigrationStates = map[models.ContentMigrationWorkflow]bool{
	models.ContentMigrationCreated:   true,
	models.ContentMigrationCompleted: true,
	models.ContentMigrationFailed:    true,
}

func (h *ContentMigrationHandler) GetMigration(c *fiber.Ctx) error {
	migration, wrote := h.requireMigrationInCourse(c)
	if wrote {
		return nil
	}

	return c.JSON(contentMigrationToJSON(migration))
}

func (h *ContentMigrationHandler) UpdateMigration(c *fiber.Ctx) error {
	migration, wrote := h.requireMigrationInCourse(c)
	if wrote {
		return nil
	}

	var input struct {
		ContentMigration struct {
			WorkflowState     *string         `json:"workflow_state"`
			MigrationSettings json.RawMessage `json:"migration_settings"`
			Settings          json.RawMessage `json:"settings"`
		} `json:"content_migration"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ContentMigration.WorkflowState != nil {
		state := models.ContentMigrationWorkflow(*input.ContentMigration.WorkflowState)
		if !validContentMigrationStates[state] {
			return responses.BadRequest(c, "Invalid workflow_state")
		}
		migration.WorkflowState = state
	}
	raw := input.ContentMigration.MigrationSettings
	if len(raw) == 0 {
		raw = input.ContentMigration.Settings
	}
	if len(raw) > 0 {
		next := parseSettings(raw)
		// The source course was authorized at creation; it is not
		// rewritable afterwards.
		next.SourceCourseID = migration.MigrationSettings.Data().SourceCourseID
		migration.MigrationSettings = datatypes.NewJSONType(next)
	}

	if err := h.migrationService.UpdateMigration(c.Context(), migration); err != nil {
		return responses.InternalError(c, "Could not update content migration")
	}

	return c.JSON(contentMigrationToJSON(migration))
}
