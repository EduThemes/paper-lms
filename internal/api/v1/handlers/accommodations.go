package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AccommodationHandler struct {
	accommodationService *service.AccommodationService
	assignmentService    *service.AssignmentService
	authz                *ResourceAuthorizer
}

func NewAccommodationHandler(accommodationService *service.AccommodationService, assignmentService *service.AssignmentService, authz *ResourceAuthorizer) *AccommodationHandler {
	return &AccommodationHandler{
		accommodationService: accommodationService,
		assignmentService:    assignmentService,
		authz:                authz,
	}
}

func accommodationToJSON(a *models.StudentAccommodation) fiber.Map {
	return fiber.Map{
		"id":                 a.ID,
		"user_id":            a.UserID,
		"course_id":          a.CourseID,
		"accommodation_type": a.AccommodationType,
		"description":        a.Description,
		"time_multiplier":    a.TimeMultiplier,
		"extra_days":         a.ExtraDays,
		"status":             a.Status,
		"plan_type":          a.PlanType,
		"plan_external_id":   a.PlanExternalID,
		"created_by_id":      a.CreatedByID,
		"approved_by_id":     a.ApprovedByID,
		"approved_at":        a.ApprovedAt,
		"effective_from":     a.EffectiveFrom,
		"effective_until":    a.EffectiveUntil,
		"notes":              a.Notes,
		"created_at":         a.CreatedAt,
		"updated_at":         a.UpdatedAt,
	}
}

// ListUserAccommodations handles GET /api/v1/users/:user_id/accommodations
func (h *AccommodationHandler) ListUserAccommodations(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.accommodationService.ListStudentAccommodations(c.Context(), uint(userID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch accommodations")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, a := range result.Items {
		items[i] = accommodationToJSON(&a)
	}

	return c.JSON(items)
}

// CreateAccommodation handles POST /api/v1/users/:user_id/accommodations
func (h *AccommodationHandler) CreateAccommodation(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		Accommodation struct {
			CourseID          *uint      `json:"course_id"`
			AccommodationType string     `json:"accommodation_type"`
			Description       string     `json:"description"`
			TimeMultiplier    *float64   `json:"time_multiplier"`
			ExtraDays         *int       `json:"extra_days"`
			PlanType          string     `json:"plan_type"`
			PlanExternalID    string     `json:"plan_external_id"`
			EffectiveFrom     *time.Time `json:"effective_from"`
			EffectiveUntil    *time.Time `json:"effective_until"`
			Notes             string     `json:"notes"`
			CreatedByID       uint       `json:"created_by_id"`
		} `json:"accommodation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	effectiveFrom := time.Now()
	if input.Accommodation.EffectiveFrom != nil {
		effectiveFrom = *input.Accommodation.EffectiveFrom
	}

	accommodation := &models.StudentAccommodation{
		UserID:            uint(userID),
		CourseID:          input.Accommodation.CourseID,
		AccommodationType: input.Accommodation.AccommodationType,
		Description:       input.Accommodation.Description,
		TimeMultiplier:    input.Accommodation.TimeMultiplier,
		ExtraDays:         input.Accommodation.ExtraDays,
		PlanType:          input.Accommodation.PlanType,
		PlanExternalID:    input.Accommodation.PlanExternalID,
		EffectiveFrom:     effectiveFrom,
		EffectiveUntil:    input.Accommodation.EffectiveUntil,
		Notes:             input.Accommodation.Notes,
		CreatedByID:       input.Accommodation.CreatedByID,
	}

	if err := h.accommodationService.CreateAccommodation(c.Context(), accommodation); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(accommodationToJSON(accommodation))
}

// GetAccommodation handles GET /api/v1/accommodations/:id
func (h *AccommodationHandler) GetAccommodation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid accommodation ID")
	}

	accommodation, err := h.accommodationService.GetAccommodation(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "accommodation")
	}

	// Authorization: only the accommodation's user or an admin can view it
	if err := h.authz.RequireOwnerOrAdmin(c, accommodation.UserID); err != nil {
		return err
	}

	return c.JSON(accommodationToJSON(accommodation))
}

// UpdateAccommodation handles PUT /api/v1/accommodations/:id
func (h *AccommodationHandler) UpdateAccommodation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid accommodation ID")
	}

	accommodation, err := h.accommodationService.GetAccommodation(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "accommodation")
	}

	var input struct {
		Accommodation struct {
			CourseID          *uint      `json:"course_id"`
			AccommodationType *string    `json:"accommodation_type"`
			Description       *string    `json:"description"`
			TimeMultiplier    *float64   `json:"time_multiplier"`
			ExtraDays         *int       `json:"extra_days"`
			Status            *string    `json:"status"`
			PlanType          *string    `json:"plan_type"`
			PlanExternalID    *string    `json:"plan_external_id"`
			ApprovedByID      *uint      `json:"approved_by_id"`
			EffectiveFrom     *time.Time `json:"effective_from"`
			EffectiveUntil    *time.Time `json:"effective_until"`
			Notes             *string    `json:"notes"`
		} `json:"accommodation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Accommodation.CourseID != nil {
		accommodation.CourseID = input.Accommodation.CourseID
	}
	if input.Accommodation.AccommodationType != nil {
		accommodation.AccommodationType = *input.Accommodation.AccommodationType
	}
	if input.Accommodation.Description != nil {
		accommodation.Description = *input.Accommodation.Description
	}
	if input.Accommodation.TimeMultiplier != nil {
		accommodation.TimeMultiplier = input.Accommodation.TimeMultiplier
	}
	if input.Accommodation.ExtraDays != nil {
		accommodation.ExtraDays = input.Accommodation.ExtraDays
	}
	if input.Accommodation.Status != nil {
		accommodation.Status = *input.Accommodation.Status
	}
	if input.Accommodation.PlanType != nil {
		accommodation.PlanType = *input.Accommodation.PlanType
	}
	if input.Accommodation.PlanExternalID != nil {
		accommodation.PlanExternalID = *input.Accommodation.PlanExternalID
	}
	if input.Accommodation.ApprovedByID != nil {
		accommodation.ApprovedByID = input.Accommodation.ApprovedByID
		now := time.Now()
		accommodation.ApprovedAt = &now
	}
	if input.Accommodation.EffectiveFrom != nil {
		accommodation.EffectiveFrom = *input.Accommodation.EffectiveFrom
	}
	if input.Accommodation.EffectiveUntil != nil {
		accommodation.EffectiveUntil = input.Accommodation.EffectiveUntil
	}
	if input.Accommodation.Notes != nil {
		accommodation.Notes = *input.Accommodation.Notes
	}

	if err := h.accommodationService.UpdateAccommodation(c.Context(), accommodation); err != nil {
		return responses.InternalError(c, "Could not update accommodation")
	}

	return c.JSON(accommodationToJSON(accommodation))
}

// DeleteAccommodation handles DELETE /api/v1/accommodations/:id
func (h *AccommodationHandler) DeleteAccommodation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid accommodation ID")
	}

	if err := h.accommodationService.DeactivateAccommodation(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not deactivate accommodation")
	}

	return c.JSON(fiber.Map{"deactivated": true})
}

// ListCourseAccommodations handles GET /api/v1/courses/:course_id/accommodations
func (h *AccommodationHandler) ListCourseAccommodations(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// List all accommodations that apply to this course (course-specific + global)
	// We use a broad query via the service layer
	result, err := h.accommodationService.ListStudentAccommodations(c.Context(), 0, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch accommodations")
	}

	// Filter to only those that apply to this course or are global
	cid := uint(courseID)
	filtered := make([]fiber.Map, 0)
	for _, a := range result.Items {
		if a.CourseID == nil || *a.CourseID == cid {
			filtered = append(filtered, accommodationToJSON(&a))
		}
	}

	return c.JSON(filtered)
}

// ApplyAccommodationsToAssignment handles POST /api/v1/assignments/:id/apply_accommodations
// This previews what accommodations would be applied to an assignment for specified users.
func (h *AccommodationHandler) ApplyAccommodationsToAssignment(c *fiber.Ctx) error {
	assignmentID, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(assignmentID), callerAccountID(c))
	if err != nil {
		return responses.NotFound(c, "assignment")
	}

	var input struct {
		UserIDs []uint `json:"user_ids"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.UserIDs) == 0 {
		return responses.BadRequest(c, "user_ids is required")
	}

	adjustments := make([]fiber.Map, 0)
	courseID := assignment.CourseID

	for _, userID := range input.UserIDs {
		adjustment, err := h.accommodationService.ApplyAccommodationsToAssignment(c.Context(), userID, &courseID, assignment.DueAt)
		if err != nil {
			continue
		}
		if adjustment != nil {
			adjustments = append(adjustments, fiber.Map{
				"user_id":          userID,
				"assignment_id":    assignmentID,
				"accommodation_id": adjustment.AccommodationID,
				"original_due_at":  adjustment.OriginalDueAt,
				"adjusted_due_at":  adjustment.AdjustedDueAt,
				"extra_days":       adjustment.ExtraDays,
			})
		}
	}

	return c.JSON(fiber.Map{
		"assignment_id": assignmentID,
		"adjustments":   adjustments,
	})
}
