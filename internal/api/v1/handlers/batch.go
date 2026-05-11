package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type BatchHandler struct {
	batchService *service.BatchService
	authz        *ResourceAuthorizer
}

func NewBatchHandler(batchService *service.BatchService, authz *ResourceAuthorizer) *BatchHandler {
	return &BatchHandler{batchService: batchService, authz: authz}
}

// CloneCourse handles POST /api/v1/courses/clone
func (h *BatchHandler) CloneCourse(c *fiber.Ctx) error {
	var input struct {
		SourceCourseID uint   `json:"source_course_id"`
		Name           string `json:"name"`
		AccountID      uint   `json:"account_id"`
		Include        struct {
			Modules     bool `json:"modules"`
			Assignments bool `json:"assignments"`
			Pages       bool `json:"pages"`
			Quizzes     bool `json:"quizzes"`
			Discussions bool `json:"discussions"`
		} `json:"include"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.SourceCourseID == 0 {
		return responses.BadRequest(c, "source_course_id is required")
	}
	if input.Name == "" {
		return responses.BadRequest(c, "name is required")
	}
	if input.AccountID == 0 {
		input.AccountID = 1 // default account
	}

	course, err := h.batchService.CloneCourse(
		c.Context(),
		input.SourceCourseID,
		input.Name,
		input.AccountID,
		input.Include.Modules,
		input.Include.Assignments,
		input.Include.Pages,
		input.Include.Quizzes,
		input.Include.Discussions,
	)
	if err != nil {
		return responses.InternalError(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":             course.ID,
		"name":           course.Name,
		"course_code":    course.CourseCode,
		"account_id":     course.AccountID,
		"workflow_state": course.WorkflowState,
		"created_at":     course.CreatedAt,
	})
}

// BulkDateShift handles POST /api/v1/courses/:course_id/date_shift
func (h *BatchHandler) BulkDateShift(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		OldStartDate string `json:"old_start_date"`
		NewStartDate string `json:"new_start_date"`
		DayShift     int    `json:"day_shift"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	var oldStart, newStart time.Time

	if input.OldStartDate != "" {
		parsed, err := time.Parse("2006-01-02", input.OldStartDate)
		if err != nil {
			return responses.BadRequest(c, "Invalid old_start_date format, expected YYYY-MM-DD")
		}
		oldStart = parsed
	}

	if input.NewStartDate != "" {
		parsed, err := time.Parse("2006-01-02", input.NewStartDate)
		if err != nil {
			return responses.BadRequest(c, "Invalid new_start_date format, expected YYYY-MM-DD")
		}
		newStart = parsed
	}

	if input.OldStartDate == "" && input.NewStartDate == "" && input.DayShift == 0 {
		return responses.BadRequest(c, "Either old_start_date/new_start_date or day_shift is required")
	}

	result, err := h.batchService.BulkDateShift(
		c.Context(),
		uint(courseID),
		oldStart,
		newStart,
		input.DayShift,
	)
	if err != nil {
		return responses.InternalError(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"assignments_shifted": result.AssignmentsShifted,
		"events_shifted":      result.EventsShifted,
		"day_shift":           result.DayShift,
	})
}

// BulkSendMessage handles POST /api/v1/conversations/bulk
func (h *BatchHandler) BulkSendMessage(c *fiber.Ctx) error {
	userID, _ := c.Locals("user_id").(uint)

	var input struct {
		CourseID        uint     `json:"course_id"`
		EnrollmentTypes []string `json:"enrollment_types"`
		Subject         string   `json:"subject"`
		Body            string   `json:"body"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if input.CourseID == 0 {
		return responses.BadRequest(c, "course_id is required")
	}
	if input.Subject == "" {
		return responses.BadRequest(c, "subject is required")
	}
	if input.Body == "" {
		return responses.BadRequest(c, "body is required")
	}
	if len(input.EnrollmentTypes) == 0 {
		return responses.BadRequest(c, "enrollment_types is required")
	}

	// Authorization: only instructors in the target course can bulk-send messages
	if err := h.authz.RequireCourseInstructor(c, input.CourseID); err != nil {
		return err
	}

	result, err := h.batchService.BulkSendMessage(
		c.Context(),
		userID,
		input.CourseID,
		input.EnrollmentTypes,
		input.Subject,
		input.Body,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"messages_sent": result.MessagesSent,
		"errors":        result.Errors,
	})
}

// BulkEnrollUsers handles POST /api/v1/courses/:course_id/enrollments/bulk
func (h *BatchHandler) BulkEnrollUsers(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Enrollments []service.BulkEnrollmentRequest `json:"enrollments"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.Enrollments) == 0 {
		return responses.BadRequest(c, "enrollments array is required")
	}

	result, err := h.batchService.BulkEnrollUsers(
		c.Context(),
		uint(courseID),
		input.Enrollments,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"enrolled": result.Enrolled,
		"errors":   result.Errors,
	})
}

// BulkUpdateAssignmentDates handles POST /api/v1/courses/:course_id/assignments/bulk_update_dates
func (h *BatchHandler) BulkUpdateAssignmentDates(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	var input struct {
		Updates []service.AssignmentDateUpdate `json:"updates"`
	}

	if err := c.BodyParser(&input); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	if len(input.Updates) == 0 {
		return responses.BadRequest(c, "updates array is required")
	}

	result, err := h.batchService.BulkUpdateAssignmentDates(
		c.Context(),
		uint(courseID),
		input.Updates,
	)
	if err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.JSON(fiber.Map{
		"updated": result.Updated,
		"errors":  result.Errors,
	})
}
