package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ContentImportHandler handles IMSCC/Common Cartridge file upload and import.
type ContentImportHandler struct {
	imsccParser             *service.IMSCCParser
	contentMigrationService *service.ContentMigrationService
	fileStoragePath         string
	// post applies the selective-import / date-shift options after a
	// package is parsed (G067). Optional.
	post *service.ContentMigrationRunner
}

// SetPostProcessor wires the import options (type selection, date
// shifting) applied after a package is parsed.
func (h *ContentImportHandler) SetPostProcessor(r *service.ContentMigrationRunner) {
	h.post = r
}

// NewContentImportHandler creates a new handler for content package imports.
func NewContentImportHandler(
	parser *service.IMSCCParser,
	migrationService *service.ContentMigrationService,
	fileStoragePath string,
) *ContentImportHandler {
	return &ContentImportHandler{
		imsccParser:             parser,
		contentMigrationService: migrationService,
		fileStoragePath:         fileStoragePath,
	}
}

// ImportPackage accepts a multipart file upload of an IMSCC/Common Cartridge zip,
// creates a content migration record, parses the package, and returns the import result.
func (h *ContentImportHandler) ImportPackage(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// Get user ID from auth context
	userID, _ := c.Locals("user_id").(uint)

	// Get the uploaded file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File upload is required. Use multipart form field 'file'.")
	}

	// Optional import options as a JSON form field "settings":
	// {"include_only_types": [...], "exclude_types": [...],
	//  "date_shift_options": {...}}.
	var settings models.MigrationSettings
	if raw := c.FormValue("settings"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &settings); err != nil {
			return responses.BadRequest(c, "Invalid settings: "+err.Error())
		}
		// A package import has no source course.
		settings.SourceCourseID = nil
	}

	// Validate file extension
	ext := filepath.Ext(fileHeader.Filename)
	if ext != ".imscc" && ext != ".zip" {
		return responses.BadRequest(c, "Invalid file type. Only .imscc and .zip files are accepted.")
	}

	// Create uploads directory
	uploadsDir := filepath.Join(h.fileStoragePath, "content_migrations", fmt.Sprintf("%d", courseID))
	if err := os.MkdirAll(uploadsDir, 0o750); err != nil {
		return responses.InternalError(c, "Could not create upload directory")
	}

	// Save the uploaded file with a unique name to avoid collisions
	uniqueName := fmt.Sprintf("%s_%s", uuid.New().String(), fileHeader.Filename)
	zipPath := filepath.Join(uploadsDir, uniqueName)

	if err := c.SaveFile(fileHeader, zipPath); err != nil {
		return responses.InternalError(c, "Could not save uploaded file")
	}

	// Determine migration type from file extension
	migrationType := "common_cartridge"
	if ext == ".imscc" {
		migrationType = "canvas_cartridge"
	}

	// Create content migration record
	now := time.Now()
	migration := &models.ContentMigration{
		CourseID:          uint(courseID),
		UserID:            userID,
		MigrationType:     migrationType,
		WorkflowState:     models.ContentMigrationRunning,
		Progress:          0,
		Attachment:        zipPath,
		StartedAt:         &now,
		MigrationSettings: datatypes.NewJSONType(settings),
	}

	if err := h.contentMigrationService.CreateMigration(c.Context(), migration); err != nil {
		// Clean up the uploaded file on failure
		_ = os.Remove(zipPath)
		return responses.InternalError(c, "Could not create content migration record")
	}

	// Parse the package
	importResult, parseErr := h.imsccParser.ParsePackage(c.Context(), uint(courseID), userID, zipPath)

	// Update migration status based on result
	finishedAt := time.Now()
	migration.FinishedAt = &finishedAt

	if parseErr != nil {
		// Roll back any rows that were written before the failure so the
		// course doesn't end up half-populated.
		if importResult != nil {
			h.imsccParser.CleanupFailedImport(c.Context(), uint(courseID), importResult.CreatedEntities)
		}

		migration.WorkflowState = models.ContentMigrationFailed
		migration.ErrorMessage = parseErr.Error()
		migration.Progress = 0
		if err := h.contentMigrationService.UpdateMigration(c.Context(), migration); err != nil {
			slog.Error("content import: update failed migration record", "migration_id", migration.ID, "error", err)
		}

		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": []fiber.Map{
				{"message": fmt.Sprintf("Import failed: %s", parseErr.Error())},
			},
			"migration_id": migration.ID,
		})
	}

	// Check if there were any import-level errors
	if len(importResult.Errors) > 0 && importResult.ModulesCreated == 0 &&
		importResult.PagesCreated == 0 && importResult.AssignmentsCreated == 0 &&
		importResult.QuizzesCreated == 0 && importResult.DiscussionsCreated == 0 {
		// Whole import failed — roll back the partial writes too.
		h.imsccParser.CleanupFailedImport(c.Context(), uint(courseID), importResult.CreatedEntities)
		migration.WorkflowState = models.ContentMigrationFailed
		migration.ErrorMessage = fmt.Sprintf("All imports failed. First error: %s", importResult.Errors[0])
		migration.Progress = 0
	} else {
		if h.post != nil {
			importResult.Warnings = append(importResult.Warnings, h.post.PostProcessImport(c.Context(), uint(courseID), settings, importResult)...)
		}
		migration.WorkflowState = models.ContentMigrationCompleted
		migration.Progress = 100
		migration.Results = datatypes.NewJSONType(*service.ImportResultSummary(importResult))
	}

	if err := h.contentMigrationService.UpdateMigration(c.Context(), migration); err != nil {
		slog.Error("content import: update migration record", "migration_id", migration.ID, "error", err)
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"migration_id":         migration.ID,
		"workflow_state":       migration.WorkflowState,
		"modules_created":      importResult.ModulesCreated,
		"pages_created":        importResult.PagesCreated,
		"assignments_created":  importResult.AssignmentsCreated,
		"quizzes_created":      importResult.QuizzesCreated,
		"questions_created":    importResult.QuestionsCreated,
		"discussions_created":  importResult.DiscussionsCreated,
		"module_items_created": importResult.ModuleItemsCreated,
		"errors":               importResult.Errors,
		"warnings":             importResult.Warnings,
	})
}
