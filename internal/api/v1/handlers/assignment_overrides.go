package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AssignmentOverrideHandler struct {
	overrideService *service.OverrideService
}

func NewAssignmentOverrideHandler(overrideService *service.OverrideService) *AssignmentOverrideHandler {
	return &AssignmentOverrideHandler{overrideService: overrideService}
}

func assignmentOverrideToJSON(o *models.AssignmentOverride) fiber.Map {
	return fiber.Map{
		"id":                o.ID,
		"assignment_id":     o.AssignmentID,
		"title":             o.Title,
		"due_at":            o.DueAt,
		"unlock_at":         o.UnlockAt,
		"lock_at":           o.LockAt,
		"all_day":           o.AllDay,
		"all_day_date":      o.AllDayDate,
		"course_section_id": o.CourseSectionID,
		"workflow_state":    o.WorkflowState,
		"created_at":        o.CreatedAt,
		"updated_at":        o.UpdatedAt,
	}
}

func (h *AssignmentOverrideHandler) ListOverrides(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrides, err := h.overrideService.ListOverrides(c.Context(), uint(assignmentID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch assignment overrides")
	}

	result := make([]fiber.Map, len(overrides))
	for i, o := range overrides {
		// Fetch student IDs for each override
		students, _ := h.overrideService.ListStudents(c.Context(), o.ID)
		studentIDs := make([]uint, len(students))
		for j, s := range students {
			studentIDs[j] = s.UserID
		}

		overrideJSON := assignmentOverrideToJSON(&o)
		overrideJSON["student_ids"] = studentIDs
		result[i] = overrideJSON
	}

	return c.JSON(result)
}

func (h *AssignmentOverrideHandler) CreateOverride(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	var input struct {
		AssignmentOverride struct {
			Title           string     `json:"title"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			AllDay          bool       `json:"all_day"`
			AllDayDate      *time.Time `json:"all_day_date"`
			CourseSectionID *uint      `json:"course_section_id"`
			StudentIDs      []uint     `json:"student_ids"`
		} `json:"assignment_override"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	override := &models.AssignmentOverride{
		AssignmentID:    uint(assignmentID),
		Title:           input.AssignmentOverride.Title,
		DueAt:           input.AssignmentOverride.DueAt,
		UnlockAt:        input.AssignmentOverride.UnlockAt,
		LockAt:          input.AssignmentOverride.LockAt,
		AllDay:          input.AssignmentOverride.AllDay,
		AllDayDate:      input.AssignmentOverride.AllDayDate,
		CourseSectionID: input.AssignmentOverride.CourseSectionID,
	}

	if err := h.overrideService.CreateOverride(c.Context(), override); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	// Add students if provided
	for _, studentID := range input.AssignmentOverride.StudentIDs {
		_ = h.overrideService.AddStudent(c.Context(), override.ID, studentID, uint(assignmentID))
	}

	// Build response with student IDs
	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = input.AssignmentOverride.StudentIDs

	return c.Status(fiber.StatusCreated).JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) GetOverride(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}

	// Verify the override belongs to the URL's assignment (prevents cross-course IDOR)
	if override.AssignmentID != uint(assignmentID) {
		return responses.NotFound(c, "assignment override")
	}

	// Fetch student IDs
	students, _ := h.overrideService.ListStudents(c.Context(), override.ID)
	studentIDs := make([]uint, len(students))
	for i, s := range students {
		studentIDs[i] = s.UserID
	}

	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = studentIDs

	return c.JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) UpdateOverride(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}

	// Verify the override belongs to the URL's assignment (prevents cross-course IDOR)
	if override.AssignmentID != uint(assignmentID) {
		return responses.NotFound(c, "assignment override")
	}

	var input struct {
		AssignmentOverride struct {
			Title           *string    `json:"title"`
			DueAt           *time.Time `json:"due_at"`
			UnlockAt        *time.Time `json:"unlock_at"`
			LockAt          *time.Time `json:"lock_at"`
			AllDay          *bool      `json:"all_day"`
			AllDayDate      *time.Time `json:"all_day_date"`
			CourseSectionID *uint      `json:"course_section_id"`
		} `json:"assignment_override"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.AssignmentOverride.Title != nil {
		override.Title = *input.AssignmentOverride.Title
	}
	if input.AssignmentOverride.DueAt != nil {
		override.DueAt = input.AssignmentOverride.DueAt
	}
	if input.AssignmentOverride.UnlockAt != nil {
		override.UnlockAt = input.AssignmentOverride.UnlockAt
	}
	if input.AssignmentOverride.LockAt != nil {
		override.LockAt = input.AssignmentOverride.LockAt
	}
	if input.AssignmentOverride.AllDay != nil {
		override.AllDay = *input.AssignmentOverride.AllDay
	}
	if input.AssignmentOverride.AllDayDate != nil {
		override.AllDayDate = input.AssignmentOverride.AllDayDate
	}
	if input.AssignmentOverride.CourseSectionID != nil {
		override.CourseSectionID = input.AssignmentOverride.CourseSectionID
	}

	if err := h.overrideService.UpdateOverride(c.Context(), override); err != nil {
		return responses.InternalError(c, "Could not update assignment override")
	}

	// Fetch student IDs
	students, _ := h.overrideService.ListStudents(c.Context(), override.ID)
	studentIDs := make([]uint, len(students))
	for i, s := range students {
		studentIDs[i] = s.UserID
	}

	overrideJSON := assignmentOverrideToJSON(override)
	overrideJSON["student_ids"] = studentIDs

	return c.JSON(overrideJSON)
}

func (h *AssignmentOverrideHandler) DeleteOverride(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	overrideID, err := c.ParamsInt("override_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid override ID")
	}

	// Verify the override belongs to the URL's assignment before deleting
	override, err := h.overrideService.GetOverride(c.Context(), uint(overrideID))
	if err != nil {
		return responses.NotFound(c, "assignment override")
	}
	if override.AssignmentID != uint(assignmentID) {
		return responses.NotFound(c, "assignment override")
	}

	if err := h.overrideService.DeleteOverride(c.Context(), uint(overrideID)); err != nil {
		return responses.InternalError(c, "Could not delete assignment override")
	}

	return c.JSON(fiber.Map{"delete": true})
}
