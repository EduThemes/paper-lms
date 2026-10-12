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
	// Wave 5.2: 13.5 LogPIIAccess sweep. Accommodation rows reveal
	// disability / 504 / IEP plan information about a student — the
	// most sensitive FERPA-protected data the LMS holds. Every read
	// where the caller is not the affected student fires a PII access
	// log row. Nil is safe (no-op) so legacy tests don't break.
	auditService *service.AuditService
	// gradingPolicy (optional): accommodation extensions are part of each
	// student's effective due date, so late / missing statuses are
	// refreshed when an accommodation changes (G040).
	gradingPolicy *service.GradingPolicyService
}

// SetGradingPolicyService wires effective-date resolution and the
// late / missing refresh.
func (h *AccommodationHandler) SetGradingPolicyService(p *service.GradingPolicyService) {
	h.gradingPolicy = p
}

// refreshStudent recomputes the student's late / missing statuses.
func (h *AccommodationHandler) refreshStudent(c *fiber.Ctx, a *models.StudentAccommodation) {
	if h.gradingPolicy == nil || a == nil {
		return
	}
	_ = h.gradingPolicy.RefreshUserDueStatus(c.Context(), a.UserID, a.CourseID)
}

func NewAccommodationHandler(accommodationService *service.AccommodationService, assignmentService *service.AssignmentService, authz *ResourceAuthorizer, auditService *service.AuditService) *AccommodationHandler {
	return &AccommodationHandler{
		accommodationService: accommodationService,
		assignmentService:    assignmentService,
		authz:                authz,
		auditService:         auditService,
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

	// Wave 5.2 FERPA audit: reading another user's accommodation list
	// exposes disability/504/IEP plan PII. Self-reads (a student
	// reading their own accommodations) are not logged — only
	// staff/admin/observer-style accesses where the caller is not
	// the affected student.
	if callerID, _ := getUserID(c); callerID != 0 && uint(userID) != callerID && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, uint(userID), "read", "accommodations_list", "student_accommodations", uint(userID), c.IP(), c.Get("User-Agent"))
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
		} `json:"accommodation"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// The `admin` gate checks the tenant only on /accounts/:account_id
	// paths: the target student (and any course) must be in the caller's
	// tenant, or this is a 404.
	if err := h.authz.RequireUserInTenant(c, uint(userID)); err != nil {
		return err
	}
	if cid := input.Accommodation.CourseID; cid != nil && *cid != 0 {
		if err := h.authz.RequireCourseInTenant(c, *cid); err != nil {
			return err
		}
	}
	callerID, err := getUserID(c)
	if err != nil {
		return err
	}

	effectiveFrom := time.Now()
	if input.Accommodation.EffectiveFrom != nil {
		effectiveFrom = *input.Accommodation.EffectiveFrom
	}

	accommodation := &models.StudentAccommodation{
		UserID:            uint(userID),
		CourseID:          input.Accommodation.CourseID,
		AccommodationType: input.Accommodation.AccommodationType,
		Description:       service.SanitizePlainText(input.Accommodation.Description),
		TimeMultiplier:    input.Accommodation.TimeMultiplier,
		ExtraDays:         input.Accommodation.ExtraDays,
		PlanType:          input.Accommodation.PlanType,
		PlanExternalID:    input.Accommodation.PlanExternalID,
		EffectiveFrom:     effectiveFrom,
		EffectiveUntil:    input.Accommodation.EffectiveUntil,
		Notes:             input.Accommodation.Notes,
		// The creator is the authenticated caller, never a body field.
		CreatedByID: callerID,
	}

	if err := h.accommodationService.CreateAccommodation(c.Context(), accommodation); err != nil {
		return responses.BadRequest(c, err.Error())
	}
	h.refreshStudent(c, accommodation)

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

	// Wave 5.2 FERPA audit: an admin/staff member reading a student's
	// individual accommodation (disability / 504 / IEP) is the most
	// sensitive read in the LMS — log it.
	if callerID, _ := getUserID(c); callerID != 0 && accommodation.UserID != callerID && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, accommodation.UserID, "read", "accommodation_detail", "student_accommodations", accommodation.ID, c.IP(), c.Get("User-Agent"))
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
	// Cross-tenant rows are a 404 (the `admin` gate has no tenant on this path).
	if err := h.authz.RequireUserInTenant(c, accommodation.UserID); err != nil {
		return authzNotFound("accommodation")
	}
	prevCourse := accommodation.CourseID

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
		if *input.Accommodation.CourseID != 0 {
			if err := h.authz.RequireCourseInTenant(c, *input.Accommodation.CourseID); err != nil {
				return err
			}
		}
		accommodation.CourseID = input.Accommodation.CourseID
	}
	if input.Accommodation.AccommodationType != nil {
		accommodation.AccommodationType = *input.Accommodation.AccommodationType
	}
	if input.Accommodation.Description != nil {
		accommodation.Description = service.SanitizePlainText(*input.Accommodation.Description)
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
		if err := h.authz.RequireUserInTenant(c, *input.Accommodation.ApprovedByID); err != nil {
			return err
		}
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

	h.refreshStudent(c, accommodation)
	if prevCourse != nil && (accommodation.CourseID == nil || *accommodation.CourseID != *prevCourse) {
		h.refreshStudent(c, &models.StudentAccommodation{UserID: accommodation.UserID, CourseID: prevCourse})
	}
	return c.JSON(accommodationToJSON(accommodation))
}

// DeleteAccommodation handles DELETE /api/v1/accommodations/:id
func (h *AccommodationHandler) DeleteAccommodation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil {
		return responses.BadRequest(c, "Invalid accommodation ID")
	}

	accommodation, err := h.accommodationService.GetAccommodation(c.Context(), uint(id))
	if err != nil {
		return responses.NotFound(c, "accommodation")
	}
	// Cross-tenant rows are a 404 (the `admin` gate has no tenant on this path).
	if err := h.authz.RequireUserInTenant(c, accommodation.UserID); err != nil {
		return authzNotFound("accommodation")
	}

	if err := h.accommodationService.DeactivateAccommodation(c.Context(), uint(id)); err != nil {
		return responses.InternalError(c, "Could not deactivate accommodation")
	}
	h.refreshStudent(c, accommodation)

	return c.JSON(fiber.Map{"deactivated": true})
}

// ListCourseAccommodations handles GET /api/v1/courses/:course_id/accommodations
func (h *AccommodationHandler) ListCourseAccommodations(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	// Only this course's students' accommodations (course-specific or
	// global): the old path listed every user's global accommodations
	// across tenants.
	if h.gradingPolicy != nil {
		return h.listCourseStudentAccommodations(c, uint(courseID))
	}

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

	// Wave 5.2 FERPA audit: bulk read of every student's accommodations
	// in a course. studentID=0 follows the bulk-read convention
	// (single row per request rather than N).
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_accommodations_read", "courses", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(filtered)
}

// ApplyAccommodationsToAssignment handles POST /api/v1/assignments/:id/apply_accommodations
// This previews what accommodations would be applied to an assignment for specified users.
func (h *AccommodationHandler) ApplyAccommodationsToAssignment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	// B122: the route declares :assignment_id (reading "id" made every
	// call a 400).
	assignmentID, err := c.ParamsInt("assignment_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid assignment ID")
	}

	assignment, err := h.assignmentService.GetByID(c.Context(), uint(assignmentID), callerAccountID(c))
	if err != nil || assignment.CourseID != uint(courseID) {
		// F-013 parent-tie: an assignment of another course is a 404.
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
	if h.gradingPolicy != nil {
		adjustments = h.applyWithEffectiveDates(c, assignment, dedupeUints(input.UserIDs))
	} else {
		assignmentCourseID := assignment.CourseID
		for _, userID := range input.UserIDs {
			adjustment, err := h.accommodationService.ApplyAccommodationsToAssignment(c.Context(), userID, &assignmentCourseID, assignment.DueAt)
			if err != nil || adjustment == nil {
				continue
			}
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

	// Wave 5.2 FERPA audit: preview reveals per-student accommodation
	// adjustments (extra days, time multipliers). studentID=0 because
	// this is a multi-user preview keyed by an explicit user_ids list;
	// the bulk convention applies.
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_accommodation_preview", "assignments", uint(assignmentID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"assignment_id": assignmentID,
		"adjustments":   adjustments,
	})
}

// listCourseStudentAccommodations lists the accommodations in effect for
// the course's students (course-specific or global).
func (h *AccommodationHandler) listCourseStudentAccommodations(c *fiber.Ctx, courseID uint) error {
	repo := h.gradingPolicy.Repo()
	students, err := repo.ListCourseStudents(c.Context(), courseID)
	if err != nil {
		return responses.InternalError(c, "Could not fetch accommodations")
	}
	ids := make([]uint, 0, len(students))
	for _, e := range students {
		ids = append(ids, e.UserID)
	}
	accs, err := repo.ListCourseAccommodations(c.Context(), courseID, dedupeUints(ids))
	if err != nil {
		return responses.InternalError(c, "Could not fetch accommodations")
	}
	out := make([]fiber.Map, 0, len(accs))
	for i := range accs {
		out = append(out, accommodationToJSON(&accs[i]))
	}
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_accommodations_read", "courses", courseID, c.IP(), c.Get("User-Agent"))
	}
	return c.JSON(out)
}

// applyWithEffectiveDates applies extra-day accommodations on top of each
// student's override-aware due date (G040): the extension is part of the
// student's effective due and lock dates for submitting, late and missing
// statuses; the application is recorded and statuses refreshed. Users who
// are not students of the course, or have no extension, are skipped.
func (h *AccommodationHandler) applyWithEffectiveDates(c *fiber.Ctx, assignment *models.Assignment, userIDs []uint) []fiber.Map {
	out := make([]fiber.Map, 0)
	course, err := h.gradingPolicy.Repo().FindCourse(c.Context(), assignment.CourseID)
	if err != nil || course == nil || len(userIDs) == 0 {
		return out
	}
	dates, err := h.gradingPolicy.DueDates(c.Context(), course, userIDs)
	if err != nil {
		return out
	}
	courseID := assignment.CourseID
	for _, uid := range userIDs {
		d, ok := dates[assignment.ID][uid]
		if !ok || d.ExtraDays <= 0 || d.DueAt == nil {
			continue
		}
		original := d.DueAt.AddDate(0, 0, -d.ExtraDays)
		adj, err := h.accommodationService.ApplyAccommodationsToAssignment(c.Context(), uid, &courseID, &original)
		accID := uint(0)
		if err == nil && adj != nil {
			accID = adj.AccommodationID
		}
		if accID != 0 {
			_ = h.accommodationService.RecordApplication(c.Context(), accID, "Assignment", assignment.ID, uid, &original, d.DueAt, nil, nil)
		}
		out = append(out, fiber.Map{
			"user_id":          uid,
			"assignment_id":    assignment.ID,
			"accommodation_id": accID,
			"original_due_at":  original,
			"adjusted_due_at":  d.DueAt,
			"adjusted_lock_at": d.LockAt,
			"extra_days":       d.ExtraDays,
			"applied":          true,
		})
	}
	_ = h.gradingPolicy.RefreshDueStatus(c.Context(), assignment.CourseID, []uint{assignment.ID})
	return out
}
