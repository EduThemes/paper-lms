package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type PlannerHandler struct {
	plannerService *service.PlannerService
}

func NewPlannerHandler(plannerService *service.PlannerService) *PlannerHandler {
	return &PlannerHandler{plannerService: plannerService}
}

// GetPlannerItems handles GET /planner/items?start_date=&end_date=
// Returns a unified list of upcoming assignments, quizzes, calendar events,
// and personal planner notes for the authenticated student.
func (h *PlannerHandler) GetPlannerItems(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	// Default date range: 2 weeks past to 2 weeks ahead
	now := time.Now()
	startDate := now.AddDate(0, 0, -14)
	endDate := now.AddDate(0, 0, 14)

	if sd := c.Query("start_date"); sd != "" {
		parsed, parseErr := time.Parse("2006-01-02", sd)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid start_date format, expected YYYY-MM-DD")
		}
		startDate = parsed
	}
	if ed := c.Query("end_date"); ed != "" {
		parsed, parseErr := time.Parse("2006-01-02", ed)
		if parseErr != nil {
			return responses.BadRequest(c, "Invalid end_date format, expected YYYY-MM-DD")
		}
		endDate = parsed
	}

	items, err := h.plannerService.GetPlannerItems(c.Context(), userID, startDate, endDate)
	if err != nil {
		return responses.InternalError(c, "Could not fetch planner items")
	}

	// Return empty array rather than null when no items
	if items == nil {
		items = []service.PlannerItem{}
	}

	return c.JSON(items)
}

// CreateNote handles POST /planner_notes
func (h *PlannerHandler) CreateNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		Title    string     `json:"title"`
		Details  string     `json:"details"`
		TodoDate *time.Time `json:"todo_date"`
		CourseID *uint      `json:"course_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	todoDate := time.Now()
	if input.TodoDate != nil {
		todoDate = *input.TodoDate
	}

	note := &models.PlannerNote{
		UserID:   userID,
		Title:    input.Title,
		Details:  input.Details,
		TodoDate: todoDate,
		CourseID: input.CourseID,
	}

	if err := h.plannerService.CreateNote(c.Context(), note); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(plannerNoteToJSON(note))
}

// UpdateNote handles PUT /planner_notes/:id
func (h *PlannerHandler) UpdateNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	noteID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid planner note ID")
	}

	note, err := h.plannerService.GetNoteByID(c.Context(), uint(noteID))
	if err != nil {
		return responses.NotFound(c, "planner note")
	}

	if note.UserID != userID {
		return responses.Error(c, fiber.StatusForbidden, "You can only update your own planner notes")
	}

	var input struct {
		Title    *string    `json:"title"`
		Details  *string    `json:"details"`
		TodoDate *time.Time `json:"todo_date"`
		CourseID *uint      `json:"course_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Title != nil {
		note.Title = *input.Title
	}
	if input.Details != nil {
		note.Details = *input.Details
	}
	if input.TodoDate != nil {
		note.TodoDate = *input.TodoDate
	}
	if input.CourseID != nil {
		note.CourseID = input.CourseID
	}

	if err := h.plannerService.UpdateNote(c.Context(), note); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(plannerNoteToJSON(note))
}

// DeleteNote handles DELETE /planner_notes/:id
func (h *PlannerHandler) DeleteNote(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	noteID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid planner note ID")
	}

	if err := h.plannerService.DeleteNote(c.Context(), uint(noteID), userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"delete": true})
}

// CreateOrUpdateOverride handles PUT /planner/overrides
func (h *PlannerHandler) CreateOrUpdateOverride(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	var input struct {
		PlannableType  string `json:"plannable_type"`
		PlannableID    uint   `json:"plannable_id"`
		MarkedComplete bool   `json:"marked_complete"`
		Dismissed      bool   `json:"dismissed"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	override := &models.PlannerOverride{
		UserID:         userID,
		PlannableType:  input.PlannableType,
		PlannableID:    input.PlannableID,
		MarkedComplete: input.MarkedComplete,
		Dismissed:      input.Dismissed,
	}

	if err := h.plannerService.CreateOrUpdateOverride(c.Context(), override); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(plannerOverrideToJSON(override))
}

// DeleteOverride handles DELETE /planner/overrides/:id
func (h *PlannerHandler) DeleteOverride(c *fiber.Ctx) error {
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	overrideID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid planner override ID")
	}

	if err := h.plannerService.DeleteOverride(c.Context(), uint(overrideID), userID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{"delete": true})
}

// --- JSON serialisation helpers ---

func plannerNoteToJSON(note *models.PlannerNote) fiber.Map {
	return fiber.Map{
		"id":             note.ID,
		"user_id":        note.UserID,
		"title":          note.Title,
		"details":        note.Details,
		"todo_date":      note.TodoDate,
		"course_id":      note.CourseID,
		"workflow_state":  note.WorkflowState,
		"created_at":     note.CreatedAt,
		"updated_at":     note.UpdatedAt,
	}
}

func plannerOverrideToJSON(override *models.PlannerOverride) fiber.Map {
	return fiber.Map{
		"id":              override.ID,
		"user_id":         override.UserID,
		"plannable_type":  override.PlannableType,
		"plannable_id":    override.PlannableID,
		"marked_complete": override.MarkedComplete,
		"dismissed":       override.Dismissed,
		"created_at":      override.CreatedAt,
		"updated_at":      override.UpdatedAt,
	}
}
