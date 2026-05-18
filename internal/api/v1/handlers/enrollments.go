package handlers

import (
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

func enrollmentToJSON(e *models.Enrollment) fiber.Map {
	result := fiber.Map{
		"id":                e.ID,
		"user_id":           e.UserID,
		"course_id":         e.CourseID,
		"course_section_id": e.CourseSectionID,
		"type":              e.Type,
		"role":              e.Role,
		"enrollment_state":  e.WorkflowState,
		"created_at":        e.CreatedAt,
		"updated_at":        e.UpdatedAt,
		"last_activity_at":  e.LastActivityAt,
	}

	if e.User != nil {
		result["user"] = fiber.Map{
			"id":            e.User.ID,
			"name":          e.User.Name,
			"sortable_name": e.User.SortableName,
			"short_name":    e.User.ShortName,
			"login_id":      e.User.LoginID,
			"email":         e.User.Email,
		}
	}

	return result
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

	enrollments := make([]fiber.Map, len(result.Items))
	for i, e := range result.Items {
		enrollments[i] = enrollmentToJSON(&e)
	}

	return c.JSON(enrollments)
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

	enrollment := &models.Enrollment{
		UserID:          input.Enrollment.UserID,
		CourseID:        uint(courseID),
		CourseSectionID: input.Enrollment.CourseSectionID,
		Type:            input.Enrollment.Type,
		WorkflowState:   models.EnrollmentActive,
	}

	if err := h.enrollmentService.Create(c.Context(), enrollment, callerAccountID(c)); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(enrollmentToJSON(enrollment))
}
