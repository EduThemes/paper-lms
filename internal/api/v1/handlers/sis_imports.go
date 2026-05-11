package handlers

import (
	"bytes"
	"io"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type SISImportHandler struct {
	sisService *service.SISImportService
}

func NewSISImportHandler(sisService *service.SISImportService) *SISImportHandler {
	return &SISImportHandler{sisService: sisService}
}

func (h *SISImportHandler) CreateSISImport(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	importType := c.FormValue("import_type")
	if importType == "" {
		return responses.BadRequest(c, "import_type is required (users, courses, sections, enrollments)")
	}

	fileHeader, err := c.FormFile("attachment")
	if err != nil {
		return responses.BadRequest(c, "attachment file is required")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return responses.InternalError(c, "Could not open uploaded file")
	}
	defer file.Close()

	// Read file into buffer so it persists beyond the request lifecycle
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, file); err != nil {
		return responses.InternalError(c, "Could not read uploaded file")
	}

	batch, err := h.sisService.CreateBatch(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not create SIS batch")
	}

	// Process synchronously
	processErr := h.sisService.ProcessImport(c.Context(), batch.ID, importType, &buf)

	// Reload batch to get final state
	batch, _ = h.sisService.GetBatch(c.Context(), batch.ID)

	if processErr != nil {
		return c.Status(fiber.StatusOK).JSON(sisBatchToJSON(batch))
	}

	return c.Status(fiber.StatusCreated).JSON(sisBatchToJSON(batch))
}

func (h *SISImportHandler) ListSISImports(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.sisService.ListBatches(c.Context(), uint(accountID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch SIS imports")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	batches := make([]fiber.Map, len(result.Items))
	for i, b := range result.Items {
		batches[i] = sisBatchToJSON(&b)
	}

	return c.JSON(batches)
}

func (h *SISImportHandler) GetSISImport(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid SIS import ID")
	}

	batch, err := h.sisService.GetBatch(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "SIS import")
	}

	return c.JSON(sisBatchToJSON(batch))
}

func (h *SISImportHandler) GetSISImportErrors(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid SIS import ID")
	}

	errors, err := h.sisService.GetBatchErrors(c.Context(), uint(id))
	if err != nil {
		return responses.InternalError(c, "Could not fetch SIS import errors")
	}

	result := make([]fiber.Map, len(errors))
	for i, e := range errors {
		result[i] = fiber.Map{
			"id":           e.ID,
			"sis_batch_id": e.SISBatchID,
			"row":          e.Row,
			"message":      e.Message,
			"file":         e.File,
			"created_at":   e.CreatedAt,
		}
	}

	return c.JSON(result)
}

func (h *SISImportHandler) ExportUsersCSV(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	data, err := h.sisService.ExportUsersCSV(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not export users CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"users.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportCoursesCSV(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	data, err := h.sisService.ExportCoursesCSV(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not export courses CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"courses.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportSectionsCSV(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	data, err := h.sisService.ExportSectionsCSV(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not export sections CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"sections.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportEnrollmentsCSV(c *fiber.Ctx) error {
	_, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	data, err := h.sisService.ExportEnrollmentsCSV(c.Context())
	if err != nil {
		return responses.InternalError(c, "Could not export enrollments CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"enrollments.csv\"")
	return c.Send(data)
}

func sisBatchToJSON(b *models.SISBatch) fiber.Map {
	return fiber.Map{
		"id":             b.ID,
		"account_id":     b.AccountID,
		"workflow_state": b.WorkflowState,
		"progress":       b.Progress,
		"data":           b.Data,
		"total_rows":     b.TotalRows,
		"processed_rows": b.ProcessedRows,
		"created_at":     b.CreatedAt,
		"updated_at":     b.UpdatedAt,
	}
}
