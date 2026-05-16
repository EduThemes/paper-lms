package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/service"
)

type GradebookHandler struct {
	gradingService *service.GradingService
	auditService   *service.AuditService
}

func NewGradebookHandler(gradingService *service.GradingService, auditService *service.AuditService) *GradebookHandler {
	return &GradebookHandler{gradingService: gradingService, auditService: auditService}
}

func (h *GradebookHandler) GetGradebook(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	gradebook, err := h.gradingService.GetGradebook(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not fetch gradebook")
	}

	// 13.5 PII audit — bulk-read semantics. A gradebook fetch on a
	// 200-student section would otherwise emit 200 audit rows per
	// page load; a single row with student_id=0 and
	// data_field="bulk_gradebook_read" captures the access pattern
	// auditors actually want (who fetched this course's gradebook).
	if callerID, _ := getUserID(c); callerID != 0 && h.auditService != nil {
		_ = h.auditService.LogPIIAccess(c.Context(), callerID, 0, "read", "bulk_gradebook_read", "gradebook", uint(courseID), c.IP(), c.Get("User-Agent"))
	}

	return c.JSON(fiber.Map{
		"students":    gradebook.Students,
		"assignments": gradebook.Assignments,
		"submissions": gradebook.Submissions,
	})
}

func (h *GradebookHandler) GetStudentGrade(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	studentID, err := c.ParamsInt("student_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid student ID")
	}

	grade, err := h.gradingService.GetStudentGrade(c.Context(), uint(courseID), uint(studentID))
	if err != nil {
		return responses.InternalError(c, "Could not calculate student grade")
	}

	return c.JSON(fiber.Map{
		"current_grade": grade.CurrentGrade,
		"current_score": grade.CurrentScore,
		"final_grade":   grade.FinalGrade,
		"final_score":   grade.FinalScore,
	})
}
