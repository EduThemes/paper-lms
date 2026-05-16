package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type ObserverHandler struct {
	observerService *service.ObserverService
	// pairingService gates LinkObservee. Pre-12.6 the route accepted
	// observee_id directly with no verification (IDOR); now the parent
	// must redeem a one-shot code minted by the student (adult-mode
	// tenants) or by a teacher in the student's course (K-12 mode).
	pairingService *service.PairingCodeService
	auditService   *service.AuditService
}

func NewObserverHandler(observerService *service.ObserverService, pairingService *service.PairingCodeService, auditService *service.AuditService) *ObserverHandler {
	return &ObserverHandler{observerService: observerService, pairingService: pairingService, auditService: auditService}
}

// LinkObservee handles POST /users/:user_id/observees
// Body: { "pairing_code": "ABC-123-XYZ" }
//
// Audit 2026-05-15 (item 12.6) — closed an IDOR where any caller could
// pass any minor's user_id as observee_id and instantly link themselves
// as observer. Now the request body must carry a freshly-minted pairing
// code; PairingCodeService.Redeem consumes the code and performs the
// observer-enrollment link inside one transaction.
func (h *ObserverHandler) LinkObservee(c *fiber.Ctx) error {
	userID, err := c.ParamsInt("user_id")
	if err != nil || userID <= 0 {
		return responses.BadRequest(c, "Invalid user ID")
	}

	var input struct {
		PairingCode string `json:"pairing_code"`
	}
	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}
	code := strings.TrimSpace(input.PairingCode)
	if code == "" {
		return responses.BadRequest(c, "pairing_code is required")
	}
	if h.pairingService == nil {
		return responses.InternalError(c, "pairing-code service unavailable")
	}

	pc, rerr := h.pairingService.Redeem(c.Context(), code, uint(userID))
	if rerr != nil {
		return responses.BadRequest(c, rerr.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          pc.UserID,
		"observer_id": userID,
		"observee_id": pc.UserID,
		"code":        pc.Code,
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

	// 13.5 PII audit — parent/observer reads child's academic
	// overview. Subject is the childID path param.
	if h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), uint(userID), uint(childID), "read", "child_overview", "child_overview", uint(childID), c.IP(), c.Get("User-Agent"))
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

	// 13.5 PII audit — parent/observer reads child's course list.
	// Subject is the observeeID path param.
	if h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), uint(userID), uint(observeeID), "read", "child_courses", "child_courses", uint(observeeID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(result)
}
