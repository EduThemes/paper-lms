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

// sisAccountScope parses :account_id and enforces it is the caller's own
// tenant (super_admin excepted). wrote=true means a 400/404 was written
// and the caller MUST short-circuit.
func sisAccountScope(c *fiber.Ctx) (uint, bool) {
	accountID, err := c.ParamsInt("account_id")
	if err != nil || accountID <= 0 {
		_ = responses.BadRequest(c, "Invalid account ID")
		return 0, true
	}
	if assertSameTenant(c, uint(accountID)) {
		return 0, true
	}
	return uint(accountID), false
}

func (h *SISImportHandler) CreateSISImport(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}

	importType := c.FormValue("import_type")
	if importType == "" {
		return responses.BadRequest(c, "import_type is required (users, courses, sections, enrollments, xlists)")
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

	batch, err := h.sisService.CreateBatch(c.Context(), accountID)
	if err != nil {
		return responses.InternalError(c, "Could not create SIS batch")
	}

	// Process synchronously
	processErr := h.sisService.ProcessImport(c.Context(), batch.ID, importType, &buf)

	// Reload batch to get final state
	if reloaded, err := h.sisService.GetBatch(c.Context(), batch.ID, accountID); err == nil {
		batch = reloaded
	}

	if processErr != nil {
		return c.Status(fiber.StatusOK).JSON(sisBatchToJSON(batch))
	}

	return c.Status(fiber.StatusCreated).JSON(sisBatchToJSON(batch))
}

func (h *SISImportHandler) ListSISImports(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}

	params := middleware.GetPagination(c)

	result, err := h.sisService.ListBatches(c.Context(), accountID, params)
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
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid SIS import ID")
	}

	// Tied to the URL account (already checked against the caller's
	// tenant): another tenant's batch id is an indistinguishable 404.
	batch, err := h.sisService.GetBatch(c.Context(), uint(id), accountID)
	if err != nil {
		return responses.NotFound(c, "SIS import")
	}

	return c.JSON(sisBatchToJSON(batch))
}

func (h *SISImportHandler) GetSISImportErrors(c *fiber.Ctx) error {
	accountID, wrote := sisAccountScope(c)
	if wrote {
		return nil
	}

	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid SIS import ID")
	}

	errors, err := h.sisService.GetBatchErrors(c.Context(), uint(id), accountID)
	if err != nil {
		return responses.NotFound(c, "SIS import")
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
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	// SEC-005: scope the export to the caller's tenant. assertSameTenant
	// writes a 404 (existence-leak contract) when a non-super-admin requests
	// another tenant's account_id, and we filter by it in the service.
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}

	data, err := h.sisService.ExportUsersCSV(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not export users CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"users.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportCoursesCSV(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}

	data, err := h.sisService.ExportCoursesCSV(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not export courses CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"courses.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportSectionsCSV(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}

	data, err := h.sisService.ExportSectionsCSV(c.Context(), uint(accountID))
	if err != nil {
		return responses.InternalError(c, "Could not export sections CSV")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"sections.csv\"")
	return c.Send(data)
}

func (h *SISImportHandler) ExportEnrollmentsCSV(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}
	if assertSameTenant(c, uint(accountID)) {
		return nil
	}

	data, err := h.sisService.ExportEnrollmentsCSV(c.Context(), uint(accountID))
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
