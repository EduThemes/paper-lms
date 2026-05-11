package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ContentExportHandler handles IMSCC content package export and download.
type ContentExportHandler struct {
	exporter        *service.IMSCCExporter
	fileStoragePath string
}

// NewContentExportHandler creates a new handler for content package exports.
func NewContentExportHandler(
	exporter *service.IMSCCExporter,
	fileStoragePath string,
) *ContentExportHandler {
	return &ContentExportHandler{
		exporter:        exporter,
		fileStoragePath: fileStoragePath,
	}
}

// ExportCourse handles POST /courses/:course_id/content_exports
// It generates an IMSCC package from the course content and returns the export result with a download URL.
func (h *ContentExportHandler) ExportCourse(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// Generate a unique export ID
	exportID := uuid.New().String()

	// Create export output directory
	outputDir := filepath.Join(h.fileStoragePath, "content_exports", fmt.Sprintf("%d", courseID), exportID)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return responses.InternalError(c, "Could not create export directory")
	}

	// Run the export
	zipPath, exportResult, exportErr := h.exporter.ExportCourse(c.Context(), uint(courseID), outputDir)
	if exportErr != nil {
		// Clean up on total failure
		os.RemoveAll(outputDir)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": []fiber.Map{
				{"message": fmt.Sprintf("Export failed: %s", exportErr.Error())},
			},
		})
	}

	// Check if anything was exported at all
	totalExported := exportResult.ModulesExported + exportResult.PagesExported +
		exportResult.AssignmentsExported + exportResult.QuizzesExported +
		exportResult.DiscussionsExported
	if totalExported == 0 && len(exportResult.Errors) > 0 {
		os.RemoveAll(outputDir)
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": []fiber.Map{
				{"message": fmt.Sprintf("Export produced no content. First error: %s", exportResult.Errors[0])},
			},
		})
	}

	// Build download URL
	downloadURL := fmt.Sprintf("/api/v1/courses/%d/content_exports/%s/download", courseID, exportID)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"export_id":            exportID,
		"course_id":            courseID,
		"download_url":         downloadURL,
		"file_path":            zipPath,
		"modules_exported":     exportResult.ModulesExported,
		"pages_exported":       exportResult.PagesExported,
		"assignments_exported": exportResult.AssignmentsExported,
		"quizzes_exported":     exportResult.QuizzesExported,
		"questions_exported":   exportResult.QuestionsExported,
		"discussions_exported": exportResult.DiscussionsExported,
		"errors":               exportResult.Errors,
	})
}

// DownloadExport handles GET /courses/:course_id/content_exports/:export_id/download
// It streams the generated IMSCC zip file as a download.
func (h *ContentExportHandler) DownloadExport(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}

	exportID := c.Params("export_id")
	if exportID == "" {
		return responses.BadRequest(c, "Invalid export ID")
	}

	// Locate the export directory
	exportDir := filepath.Join(h.fileStoragePath, "content_exports", fmt.Sprintf("%d", courseID), exportID)

	// Find the .imscc file in the export directory
	entries, err := os.ReadDir(exportDir)
	if err != nil {
		return responses.NotFound(c, "content export")
	}

	var zipFilePath string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".imscc" {
			zipFilePath = filepath.Join(exportDir, entry.Name())
			break
		}
	}

	if zipFilePath == "" {
		return responses.NotFound(c, "content export file")
	}

	// Verify the file exists and is readable
	info, err := os.Stat(zipFilePath)
	if err != nil {
		return responses.NotFound(c, "content export file")
	}

	// Sanitize filename for Content-Disposition
	fileName := filepath.Base(zipFilePath)

	// Set response headers for file download
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	return c.SendFile(zipFilePath)
}
