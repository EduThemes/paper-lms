package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// ContentExportHandler serves the course content export API (Canvas
// content_exports, G066/G142): create an IMS Common Cartridge export,
// poll it, and download the package. Every route is course-scoped and
// instructor-only; the instructor middleware ties :course_id to the
// caller's tenant, and every export is tied to :course_id.
type ContentExportHandler struct {
	exports *service.ContentExportService
}

// NewContentExportHandler creates the content export handler.
func NewContentExportHandler(exports *service.ContentExportService) *ContentExportHandler {
	return &ContentExportHandler{exports: exports}
}

func contentExportToJSON(e *models.ContentExport) fiber.Map {
	out := fiber.Map{
		"id":             e.ID,
		"course_id":      e.CourseID,
		"user_id":        e.UserID,
		"export_type":    e.ExportType,
		"workflow_state": e.WorkflowState,
		"progress":       e.Progress,
		"error_message":  e.ErrorMessage,
		"started_at":     e.StartedAt,
		"finished_at":    e.FinishedAt,
		"created_at":     e.CreatedAt,
		"updated_at":     e.UpdatedAt,
	}
	var results json.RawMessage
	if e.Results != "" && json.Unmarshal([]byte(e.Results), &results) == nil {
		out["results"] = results
	}
	var selection json.RawMessage
	if e.Selection != "" && json.Unmarshal([]byte(e.Selection), &selection) == nil {
		out["selection"] = selection
	}
	if e.WorkflowState == models.ContentExportExported {
		url := fmt.Sprintf("/api/v1/courses/%d/content_exports/%d/download", e.CourseID, e.ID)
		out["attachment"] = fiber.Map{
			"url":          url,
			"display_name": e.FileName,
			"filename":     e.FileName,
			"size":         e.FileSize,
			"content-type": "application/zip",
		}
	}
	return out
}

// requireExportInCourse loads :id and ties it to :course_id. Writes a
// 404 and returns wrote=true on any miss — the caller must return nil.
func (h *ContentExportHandler) requireExportInCourse(c *fiber.Ctx) (*models.ContentExport, bool) {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		_ = responses.BadRequest(c, "Invalid course ID")
		return nil, true
	}
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 {
		_ = responses.BadRequest(c, "Invalid content export ID")
		return nil, true
	}
	export, err := h.exports.Get(c.Context(), uint(id), callerAccountID(c))
	if err != nil || export == nil || export.CourseID != uint(courseID) {
		_ = responses.NotFound(c, "content export")
		return nil, true
	}
	return export, false
}

// ExportCourse handles POST /courses/:course_id/content_exports.
//
// Body (JSON, all optional): {"export_type": "common_cartridge",
// "skip_notifications": true, "select": {"assignments": [1]},
// "include_only_types": ["pages"], "exclude_types": ["files"]}.
// Responds 202 with the export; poll GET .../content_exports/:id until
// workflow_state is "exported", then follow attachment.url.
func (h *ContentExportHandler) ExportCourse(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		ExportType       string            `json:"export_type"`
		Select           map[string][]uint `json:"select"`
		IncludeOnlyTypes []string          `json:"include_only_types"`
		ExcludeTypes     []string          `json:"exclude_types"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&input); err != nil {
			return responses.BadRequest(c, "Invalid input")
		}
	}
	var settings *models.MigrationSettings
	if len(input.Select) > 0 || len(input.IncludeOnlyTypes) > 0 || len(input.ExcludeTypes) > 0 {
		settings = &models.MigrationSettings{
			Select:           input.Select,
			IncludeOnlyTypes: input.IncludeOnlyTypes,
			ExcludeTypes:     input.ExcludeTypes,
		}
	}
	userID, _ := c.Locals("user_id").(uint)
	export, err := h.exports.Create(c.Context(), uint(courseID), userID, strings.TrimSpace(input.ExportType), settings)
	if err != nil {
		if errors.Is(err, service.ErrInvalidExportType) {
			return responses.BadRequest(c, "export_type must be common_cartridge")
		}
		return responses.InternalError(c, "Could not start the export")
	}
	return c.Status(fiber.StatusAccepted).JSON(contentExportToJSON(export))
}

// ListExports handles GET /courses/:course_id/content_exports.
func (h *ContentExportHandler) ListExports(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil || courseID <= 0 {
		return responses.BadRequest(c, "Invalid course ID")
	}
	params := middleware.GetPagination(c)
	result, err := h.exports.List(c.Context(), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch content exports")
	}
	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)
	out := make([]fiber.Map, len(result.Items))
	for i := range result.Items {
		out[i] = contentExportToJSON(&result.Items[i])
	}
	return c.JSON(out)
}

// GetExport handles GET /courses/:course_id/content_exports/:id.
func (h *ContentExportHandler) GetExport(c *fiber.Ctx) error {
	export, wrote := h.requireExportInCourse(c)
	if wrote {
		return nil
	}
	return c.JSON(contentExportToJSON(export))
}

// DownloadExport handles GET /courses/:course_id/content_exports/:id/download
// and streams the generated .imscc package.
func (h *ContentExportHandler) DownloadExport(c *fiber.Ctx) error {
	export, wrote := h.requireExportInCourse(c)
	if wrote {
		return nil
	}
	rc, err := h.exports.Open(c.Context(), export)
	if err != nil {
		if errors.Is(err, service.ErrContentExportNotReady) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": "The export has not finished yet"}},
			})
		}
		return responses.NotFound(c, "content export file")
	}
	name := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, export.FileName)
	if name == "" {
		name = fmt.Sprintf("course_%d_export.imscc", export.CourseID)
	}
	c.Set(fiber.HeaderContentType, "application/zip")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Set("X-Content-Type-Options", "nosniff")
	size := int(export.FileSize)
	if size <= 0 {
		size = -1
	}
	// fasthttp closes the stream once the response is written.
	c.Context().SetBodyStream(rc, size)
	return nil
}
