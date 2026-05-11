package handlers

import (
	"path/filepath"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

// QTIImportHandler bridges multipart QTI / IMSCC uploads to the
// QTIImportService. Sync-only in v1 — the handler blocks for the
// duration of the parse + persist cycle. For Canvas-sized exports
// (<10MB, <1000 questions) this completes in well under a second on
// modest hardware.
type QTIImportHandler struct {
	svc *service.QTIImportService
}

func NewQTIImportHandler(svc *service.QTIImportService) *QTIImportHandler {
	return &QTIImportHandler{svc: svc}
}

// Import accepts `POST /api/v1/courses/:course_id/qti_import` with a
// multipart field named "file" containing the .imscc bundle.
//
// Returns the ImportSummary as JSON. The summary includes per-item
// warnings and errors — the HTTP status is 200 even when individual
// items failed to parse, because the bulk of the import may have
// succeeded. A 4xx/5xx response indicates the entire pipeline
// failed (bad upload, missing course, parser crash).
func (h *QTIImportHandler) Import(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	userID, _ := c.Locals("user_id").(uint)

	fh, err := c.FormFile("file")
	if err != nil {
		return responses.BadRequest(c, "File upload is required. Use multipart form field 'file'.")
	}
	ext := filepath.Ext(fh.Filename)
	if ext != ".imscc" && ext != ".zip" && ext != ".xml" {
		return responses.BadRequest(c, "Invalid file type. Accepts .imscc, .zip, or .xml.")
	}

	summary, err := h.svc.ImportMultipart(c.Context(), fh, uint(courseID), userID)
	if err != nil {
		return responses.InternalError(c, "QTI import failed: "+err.Error())
	}
	return c.Status(fiber.StatusOK).JSON(summary)
}

// Export accepts `GET /api/v1/quizzes/:quiz_id/export.imscc` and
// returns the quiz as a Canvas-Classic-compatible .imscc zip.
func (h *QTIImportHandler) Export(c *fiber.Ctx) error {
	quizID, err := c.ParamsInt("quiz_id")
	if err != nil || quizID <= 0 {
		return responses.BadRequest(c, "Invalid quiz ID")
	}
	data, err := h.svc.ExportQuiz(c.Context(), uint(quizID))
	if err != nil {
		return responses.InternalError(c, "Export failed: "+err.Error())
	}
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename=\"quiz-export.imscc\"")
	return c.Status(fiber.StatusOK).Send(data)
}
