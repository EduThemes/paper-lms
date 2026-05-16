package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

// CustomGradebookColumnHandler exposes Canvas-compatible custom gradebook
// column endpoints. Instructor-only RBAC is applied at the route level.
type CustomGradebookColumnHandler struct {
	service      *service.CustomGradebookColumnService
	auditService *service.AuditService
}

func NewCustomGradebookColumnHandler(svc *service.CustomGradebookColumnService, auditService *service.AuditService) *CustomGradebookColumnHandler {
	return &CustomGradebookColumnHandler{service: svc, auditService: auditService}
}

func customColumnToJSON(c *models.CustomGradebookColumn) fiber.Map {
	return fiber.Map{
		"id":             c.ID,
		"course_id":      c.CourseID,
		"title":          c.Title,
		"position":       c.Position,
		"hidden":         c.Hidden,
		"read_only":      c.ReadOnly,
		"teacher_notes":  c.TeacherNotes,
		"workflow_state": c.WorkflowState,
		"created_at":     c.CreatedAt,
		"updated_at":     c.UpdatedAt,
	}
}

func customColumnDatumToJSON(d *models.CustomColumnDatum) fiber.Map {
	return fiber.Map{
		"id":                         d.ID,
		"custom_gradebook_column_id": d.CustomGradebookColumnID,
		"user_id":                    d.UserID,
		"content":                    d.Content,
		"created_at":                 d.CreatedAt,
		"updated_at":                 d.UpdatedAt,
	}
}

// List ----------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) List(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	includeHidden := c.QueryBool("include_hidden", false)
	cols, err := h.service.ListColumns(c.Context(), uint(courseID), includeHidden)
	if err != nil {
		return responses.InternalError(c, "Could not fetch custom gradebook columns")
	}
	out := make([]fiber.Map, len(cols))
	for i := range cols {
		out[i] = customColumnToJSON(&cols[i])
	}
	return c.JSON(out)
}

// Create --------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) Create(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		Title        string `json:"title"`
		Position     int    `json:"position"`
		Hidden       bool   `json:"hidden"`
		ReadOnly     bool   `json:"read_only"`
		TeacherNotes bool   `json:"teacher_notes"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	col := &models.CustomGradebookColumn{
		Title:        input.Title,
		Position:     input.Position,
		Hidden:       input.Hidden,
		ReadOnly:     input.ReadOnly,
		TeacherNotes: input.TeacherNotes,
	}
	created, err := h.service.CreateColumn(c.Context(), uint(courseID), col)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.Status(fiber.StatusCreated).JSON(customColumnToJSON(created))
}

// Update --------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) Update(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	colID, err := c.ParamsInt("column_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid column ID")
	}
	var input struct {
		Title        *string `json:"title"`
		Hidden       *bool   `json:"hidden"`
		ReadOnly     *bool   `json:"read_only"`
		TeacherNotes *bool   `json:"teacher_notes"`
		Position     *int    `json:"position"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	col, err := h.service.UpdateColumn(c.Context(), uint(courseID), uint(colID),
		input.Title, input.Hidden, input.ReadOnly, input.TeacherNotes, input.Position)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(customColumnToJSON(col))
}

// Delete --------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) Delete(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	colID, err := c.ParamsInt("column_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid column ID")
	}
	if err := h.service.DeleteColumn(c.Context(), uint(courseID), uint(colID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"delete": true})
}

// Reorder -------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) Reorder(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		Order []uint `json:"order"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	if err := h.service.Reorder(c.Context(), uint(courseID), input.Order); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"reordered": true})
}

// ListData ------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) ListData(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	colID, err := c.ParamsInt("column_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid column ID")
	}
	data, err := h.service.ListData(c.Context(), uint(courseID), uint(colID))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "custom_gradebook_column_bulk", "custom_gradebook_columns", uint(colID), c.IP(), c.Get("User-Agent"))
	}
	out := make([]fiber.Map, len(data))
	for i := range data {
		out[i] = customColumnDatumToJSON(&data[i])
	}
	return c.JSON(out)
}

// SetCell -------------------------------------------------------------------

func (h *CustomGradebookColumnHandler) SetCell(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	colID, err := c.ParamsInt("column_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid column ID")
	}
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	d, err := h.service.SetCell(c.Context(), uint(courseID), uint(colID), uint(userID), input.Content)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "write", "custom_gradebook_column_cell", "custom_gradebook_columns", uint(colID), c.IP(), c.Get("User-Agent"))
	}
	return c.JSON(customColumnDatumToJSON(d))
}

// BulkUpdate ----------------------------------------------------------------

func (h *CustomGradebookColumnHandler) BulkUpdate(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	var input struct {
		Entries []service.BulkUpdateEntry `json:"entries"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	count, err := h.service.BulkUpdate(c.Context(), uint(courseID), input.Entries)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}
	return c.JSON(fiber.Map{"updated": count})
}
