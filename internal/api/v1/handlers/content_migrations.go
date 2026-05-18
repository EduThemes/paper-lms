package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"gorm.io/datatypes"
)

type ContentMigrationHandler struct {
	migrationService *service.ContentMigrationService
}

func NewContentMigrationHandler(migrationService *service.ContentMigrationService) *ContentMigrationHandler {
	return &ContentMigrationHandler{migrationService: migrationService}
}

func contentMigrationToJSON(m *models.ContentMigration) fiber.Map {
	return fiber.Map{
		"id":                 m.ID,
		"course_id":          m.CourseID,
		"user_id":            m.UserID,
		"migration_type":     m.MigrationType,
		"source_course_id":   m.SourceCourseID,
		"workflow_state":     m.WorkflowState,
		"progress":           m.Progress,
		"migration_settings": m.MigrationSettings.Data(),
		"started_at":         m.StartedAt,
		"finished_at":        m.FinishedAt,
		"error_message":      m.ErrorMessage,
		"attachment":         m.Attachment,
		"created_at":         m.CreatedAt,
		"updated_at":         m.UpdatedAt,
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

func (h *ContentMigrationHandler) CreateMigration(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		ContentMigration struct {
			MigrationType     string          `json:"migration_type"`
			SourceCourseID    *uint           `json:"source_course_id"`
			Settings          json.RawMessage `json:"settings"`
			MigrationSettings json.RawMessage `json:"migration_settings"`
		} `json:"content_migration"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	rawSettings := input.ContentMigration.Settings
	if len(rawSettings) == 0 {
		rawSettings = input.ContentMigration.MigrationSettings
	}
	settings := parseSettings(rawSettings)

	// Check for multipart file upload
	attachment := ""
	if file, fileErr := c.FormFile("attachment"); fileErr == nil && file != nil {
		uploadPath := "uploads/content_migrations/" + file.Filename
		if saveErr := c.SaveFile(file, uploadPath); saveErr != nil {
			return responses.InternalError(c, "Could not save uploaded file")
		}
		attachment = uploadPath
	}

	// Get user ID from auth context (default to 0 if not set)
	userID, _ := c.Locals("user_id").(uint)

	migration := &models.ContentMigration{
		CourseID:          uint(courseID),
		UserID:            userID,
		MigrationType:     input.ContentMigration.MigrationType,
		SourceCourseID:    input.ContentMigration.SourceCourseID,
		MigrationSettings: datatypes.NewJSONType(settings),
		Attachment:        attachment,
	}

	if err := h.migrationService.CreateMigration(c.Context(), migration); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(contentMigrationToJSON(migration))
}

func (h *ContentMigrationHandler) GetMigration(c *fiber.Ctx) error {
	migrationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid content migration ID")
	}

	migration, err := h.migrationService.GetMigration(c.Context(), uint(migrationID))
	if err != nil {
		return responses.NotFound(c, "content migration")
	}

	return c.JSON(contentMigrationToJSON(migration))
}

func (h *ContentMigrationHandler) UpdateMigration(c *fiber.Ctx) error {
	migrationID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid content migration ID")
	}

	migration, err := h.migrationService.GetMigration(c.Context(), uint(migrationID))
	if err != nil {
		return responses.NotFound(c, "content migration")
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
		migration.WorkflowState = models.ContentMigrationWorkflow(*input.ContentMigration.WorkflowState)
	}
	raw := input.ContentMigration.MigrationSettings
	if len(raw) == 0 {
		raw = input.ContentMigration.Settings
	}
	if len(raw) > 0 {
		migration.MigrationSettings = datatypes.NewJSONType(parseSettings(raw))
	}

	if err := h.migrationService.UpdateMigration(c.Context(), migration); err != nil {
		return responses.InternalError(c, "Could not update content migration")
	}

	return c.JSON(contentMigrationToJSON(migration))
}
