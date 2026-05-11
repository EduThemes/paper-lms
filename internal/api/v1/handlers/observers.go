package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ObserverHandler struct {
	observerService *service.ObserverService
}

func NewObserverHandler(observerService *service.ObserverService) *ObserverHandler {
	return &ObserverHandler{observerService: observerService}
}

// LinkObservee handles POST /users/:user_id/observees
// Body: { "observee_id": <student_user_id> }
func (h *ObserverHandler) LinkObservee(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		ObserveeID uint `json:"observee_id"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.ObserveeID == 0 {
		return responses.BadRequest(c, "observee_id is required")
	}

	if err := h.observerService.LinkObserverToStudent(c.Context(), uint(userID), input.ObserveeID); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          input.ObserveeID,
		"observer_id": userID,
		"observee_id": input.ObserveeID,
	})
}

// UnlinkObservee handles DELETE /users/:user_id/observees/:observee_id
func (h *ObserverHandler) UnlinkObservee(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	observeeID, err := c.ParamsInt("observee_id")
	if err != nil || observeeID <= 0 {
		return responses.BadRequest(c, "Invalid observee ID")
	}

	if err := h.observerService.UnlinkObserver(c.Context(), uint(userID), uint(observeeID)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"observer_id": userID,
		"observee_id": observeeID,
		"deleted":     true,
	})
}

// ListObservees handles GET /users/:user_id/observees
func (h *ObserverHandler) ListObservees(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	studentIDs, err := h.observerService.ListObservedStudents(c.Context(), uint(userID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch observees")
	}

	observees := make([]fiber.Map, len(studentIDs))
	for i, sid := range studentIDs {
		observees[i] = fiber.Map{
			"id":          sid,
			"observer_id": userID,
		}
	}

	return c.JSON(observees)
}

// GetChildOverview handles GET /users/:user_id/observees/:child_id/overview
// Returns aggregated dashboard data (courses + grades, upcoming work this
// week, recent grades, recent activity) for one observed child. The caller
// (parent) must already be linked to the child via observer enrollment.
func (h *ObserverHandler) GetChildOverview(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	childID, err := c.ParamsInt("child_id")
	if err != nil || childID <= 0 {
		return responses.BadRequest(c, "Invalid child ID")
	}

	overview, err := h.observerService.GetChildOverview(c.Context(), uint(userID), uint(childID))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(overview)
}

// GetObserveeCourses handles GET /users/:user_id/observees/:observee_id/courses
func (h *ObserverHandler) GetObserveeCourses(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	observeeID, err := c.ParamsInt("observee_id")
	if err != nil || observeeID <= 0 {
		return responses.BadRequest(c, "Invalid observee ID")
	}

	courses, err := h.observerService.GetObserveeCourses(c.Context(), uint(userID), uint(observeeID))
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	result := make([]fiber.Map, len(courses))
	for i, course := range courses {
		result[i] = fiber.Map{
			"id":             course.ID,
			"name":           course.Name,
			"course_code":    course.CourseCode,
			"workflow_state": course.WorkflowState,
		}
	}

	return c.JSON(result)
}
