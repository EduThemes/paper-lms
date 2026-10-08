package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type EnrollmentHandler struct {
	enrollmentService *service.EnrollmentService
}

func NewEnrollmentHandler(enrollmentService *service.EnrollmentService) *EnrollmentHandler {
	return &EnrollmentHandler{enrollmentService: enrollmentService}
}

// enrollmentToJSON serializes an enrollment. includeContactInfo gates the
// user's email and login_id: Canvas only shows them to roles holding
// read_email_addresses / view_user_logins (teachers, TAs and admins by
// default), never to students or observers browsing the roster (B171).
func enrollmentToJSON(e *models.Enrollment, includeContactInfo bool) fiber.Map {
	result := fiber.Map{
		"id":                e.ID,
		"user_id":           e.UserID,
		"course_id":         e.CourseID,
		"course_section_id": e.CourseSectionID,
		"type":              e.Type,
		"role":              e.Role,
		"role_id":           e.RoleID,
		"enrollment_state":  e.WorkflowState,
		"created_at":        e.CreatedAt,
		"updated_at":        e.UpdatedAt,
		"last_activity_at":  e.LastActivityAt,
	}

	if e.User != nil {
		u := fiber.Map{
			"id":            e.User.ID,
			"name":          e.User.Name,
			"sortable_name": e.User.SortableName,
			"short_name":    e.User.ShortName,
		}
		if includeContactInfo {
			u["login_id"] = e.User.LoginID
			u["email"] = e.User.Email
		}
		result["user"] = u
	}

	return result
}

// canAddEnrollmentType mirrors Canvas's default add_*_to_course grants:
// teachers and admins may add any role; TAs (and designers) may add only
// students and observers. Teacher/TA/designer enrollments carry course
// management power, so a TA must not be able to mint them.
func canAddEnrollmentType(c *fiber.Ctx, enrollmentType string) bool {
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	callerType, _ := c.Locals("enrollment_type").(string)
	switch callerType {
	case "TeacherEnrollment":
		return true
	case "TaEnrollment", "DesignerEnrollment":
		return enrollmentType == "StudentEnrollment" || enrollmentType == "ObserverEnrollment"
	}
	return false
}

// enrollmentCourseScope is the account scope for course lookups on the
// enrollment write path: the caller's tenant, or 0 (unscoped) for a
// super_admin, who crosses tenants by design.
func enrollmentCourseScope(c *fiber.Ctx) uint {
	if isSuper, _ := c.Locals("is_super_admin").(bool); isSuper {
		return 0
	}
	return callerAccountID(c)
}

func (h *EnrollmentHandler) ListEnrollments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.enrollmentService.ListByCourse(c.Context(), uint(courseID), callerAccountID(c), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch enrollments")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	includeContact := callerIsCourseStaff(c)
	enrollments := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		enrollments[i] = enrollmentToJSON(&e, includeContact)
	}

	return c.JSON(enrollments)
}

// MyCourseEnrollments handles GET /courses/:course_id/enrollments/self —
// the caller's own enrollments in the course plus whether they act as an
// admin there (B072). The UI previously scanned the first page of the
// whole roster for the caller's row; the server caps per_page at 100, so
// in a large course a later-created teacher row was never found and the
// teacher lost every editing control.
func (h *EnrollmentHandler) MyCourseEnrollments(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	userID, err := getUserID(c)
	if err != nil {
		return err
	}

	all, err := h.enrollmentService.ListByUser(c.Context(), userID, callerAccountID(c))
	if err != nil {
		return responses.InternalError(c, "Could not fetch enrollments")
	}
	mine := make([]fiber.Map, 0)
	for i := range all {
		if all[i].CourseID == uint(courseID) {
			mine = append(mine, enrollmentToJSON(&all[i], true))
		}
	}

	isAdmin, _ := c.Locals("is_admin").(bool)
	isSuper, _ := c.Locals("is_super_admin").(bool)
	return c.JSON(fiber.Map{
		"enrollments": mine,
		"is_admin":    isAdmin || isSuper,
	})
}

// SearchEnrollableUsers handles GET /courses/:course_id/enrollable_users?search_term=
// for the course "Add People" form (B073). GET /users is admin-only, so
// teachers could never find anyone to add. Matches are exact (email,
// login ID or SIS ID) within the course's tenant — see
// EnrollmentService.FindEnrollableUsers.
func (h *EnrollmentHandler) SearchEnrollableUsers(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}
	term := c.Query("search_term")
	users, err := h.enrollmentService.FindEnrollableUsers(c.Context(), uint(courseID), term, enrollmentCourseScope(c))
	if err != nil {
		return responses.NotFound(c, "course")
	}
	out := make([]fiber.Map, len(users))
	for i, u := range users {
		out[i] = fiber.Map{
			"id":            u.ID,
			"name":          u.Name,
			"sortable_name": u.SortableName,
			"login_id":      u.LoginID,
			"email":         u.Email,
		}
	}
	return c.JSON(out)
}

func (h *EnrollmentHandler) CreateEnrollment(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Enrollment struct {
			UserID          uint   `json:"user_id"`
			Type            string `json:"type"`
			CourseSectionID *uint  `json:"course_section_id"`
			RoleID          *uint  `json:"role_id"` // custom role (G102)
		} `json:"enrollment"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.Enrollment.UserID == 0 {
		return responses.BadRequest(c, "user_id is required")
	}
	if input.Enrollment.Type == "" {
		input.Enrollment.Type = "StudentEnrollment"
	}
	if !canAddEnrollmentType(c, input.Enrollment.Type) {
		return responses.Forbidden(c, "You are not allowed to add this enrollment type")
	}

	enrollment := &models.Enrollment{
		UserID:          input.Enrollment.UserID,
		CourseID:        uint(courseID),
		CourseSectionID: input.Enrollment.CourseSectionID,
		Type:            input.Enrollment.Type,
		RoleID:          input.Enrollment.RoleID,
		WorkflowState:   models.EnrollmentActive,
	}

	if err := h.enrollmentService.Create(c.Context(), enrollment, enrollmentCourseScope(c)); err != nil {
		switch {
		case errors.Is(err, service.ErrRoleNotAssignable):
			return responses.BadRequest(c, err.Error())
		case errors.Is(err, service.ErrEnrollmentUserNotFound):
			// Same 404 whether the id is unused or belongs to another
			// tenant — the endpoint must not be a cross-tenant oracle.
			return responses.NotFound(c, "user")
		case errors.Is(err, service.ErrEnrollmentSectionNotFound):
			return responses.NotFound(c, "course section")
		}
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(enrollmentToJSON(enrollment, true))
}
