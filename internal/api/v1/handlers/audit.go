package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

// AuditHandler handles HTTP requests for audit log and grade change log endpoints.
type AuditHandler struct {
	auditService *service.AuditService
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditService *service.AuditService) *AuditHandler {
	return &AuditHandler{auditService: auditService}
}

func auditLogToJSON(log *models.AuditLog) fiber.Map {
	return fiber.Map{
		"id":           log.ID,
		"event_type":   log.EventType,
		"user_id":      log.UserID,
		"course_id":    log.CourseID,
		"account_id":   log.AccountID,
		"context_type": log.ContextType,
		"context_id":   log.ContextID,
		"action":       log.Action,
		"payload":      log.Payload,
		"ip_address":   log.IPAddress,
		"user_agent":   log.UserAgent,
		"created_at":   log.CreatedAt,
	}
}

func gradeChangeLogToJSON(log *models.GradeChangeLog) fiber.Map {
	return fiber.Map{
		"id":             log.ID,
		"course_id":      log.CourseID,
		"assignment_id":  log.AssignmentID,
		"student_id":     log.StudentID,
		"grader_id":      log.GraderID,
		"submission_id":  log.SubmissionID,
		"old_grade":      log.OldGrade,
		"new_grade":      log.NewGrade,
		"old_score":      log.OldScore,
		"new_score":      log.NewScore,
		"excused":        log.Excused,
		"grading_method": log.GradingMethod,
		"created_at":     log.CreatedAt,
	}
}

// parseAuditLogFilter extracts filter parameters from query string.
func parseAuditLogFilter(c *fiber.Ctx) postgres.AuditLogFilter {
	filter := postgres.AuditLogFilter{
		EventType:   c.Query("event_type"),
		ContextType: c.Query("context_type"),
	}

	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if uid, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
			id := uint(uid)
			filter.UserID = &id
		}
	}

	if courseIDStr := c.Query("course_id"); courseIDStr != "" {
		if cid, err := strconv.ParseUint(courseIDStr, 10, 64); err == nil {
			id := uint(cid)
			filter.CourseID = &id
		}
	}

	if accountIDStr := c.Query("account_id"); accountIDStr != "" {
		if aid, err := strconv.ParseUint(accountIDStr, 10, 64); err == nil {
			id := uint(aid)
			filter.AccountID = &id
		}
	}

	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filter.DateFrom = &t
		}
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			filter.DateTo = &end
		}
	}

	return filter
}

// parseGradeChangeLogFilter extracts grade change filter parameters from query string.
func parseGradeChangeLogFilter(c *fiber.Ctx) postgres.GradeChangeLogFilter {
	filter := postgres.GradeChangeLogFilter{}

	if studentIDStr := c.Query("student_id"); studentIDStr != "" {
		if sid, err := strconv.ParseUint(studentIDStr, 10, 64); err == nil {
			id := uint(sid)
			filter.StudentID = &id
		}
	}

	if graderIDStr := c.Query("grader_id"); graderIDStr != "" {
		if gid, err := strconv.ParseUint(graderIDStr, 10, 64); err == nil {
			id := uint(gid)
			filter.GraderID = &id
		}
	}

	if assignmentIDStr := c.Query("assignment_id"); assignmentIDStr != "" {
		if aid, err := strconv.ParseUint(assignmentIDStr, 10, 64); err == nil {
			id := uint(aid)
			filter.AssignmentID = &id
		}
	}

	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			filter.DateFrom = &t
		}
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			filter.DateTo = &end
		}
	}

	return filter
}

// GetCourseAuditLog handles GET /courses/:course_id/audit_log
func (h *AuditHandler) GetCourseAuditLog(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	filter := parseAuditLogFilter(c)
	cid := uint(courseID)
	filter.CourseID = &cid

	result, err := h.auditService.GetAuditLog(c.Context(), filter, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch audit log")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	logs := make([]fiber.Map, len(result.Items))
	for i, log := range result.Items {
		logs[i] = auditLogToJSON(&log)
	}

	return c.JSON(logs)
}

// GetCourseGradeChangeLog handles GET /courses/:course_id/grade_change_log
func (h *AuditHandler) GetCourseGradeChangeLog(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	params := middleware.GetPagination(c)
	filter := parseGradeChangeLogFilter(c)
	cid := uint(courseID)
	filter.CourseID = &cid

	result, err := h.auditService.GetGradeChangeLog(c.Context(), filter, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch grade change log")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	logs := make([]fiber.Map, len(result.Items))
	for i, log := range result.Items {
		logs[i] = gradeChangeLogToJSON(&log)
	}

	return c.JSON(logs)
}

// GetAccountAuditLog handles GET /accounts/:account_id/audit_log
func (h *AuditHandler) GetAccountAuditLog(c *fiber.Ctx) error {
	accountID, err := c.ParamsInt("account_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid account ID")
	}

	params := middleware.GetPagination(c)
	filter := parseAuditLogFilter(c)
	aid := uint(accountID)
	filter.AccountID = &aid

	result, err := h.auditService.GetAuditLog(c.Context(), filter, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch audit log")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	logs := make([]fiber.Map, len(result.Items))
	for i, log := range result.Items {
		logs[i] = auditLogToJSON(&log)
	}

	return c.JSON(logs)
}

// GetAuditLogSummary handles GET /admin/audit_log/summary
func (h *AuditHandler) GetAuditLogSummary(c *fiber.Ctx) error {
	var dateFrom, dateTo *time.Time

	if dateFromStr := c.Query("date_from"); dateFromStr != "" {
		if t, err := time.Parse("2006-01-02", dateFromStr); err == nil {
			dateFrom = &t
		}
	}

	if dateToStr := c.Query("date_to"); dateToStr != "" {
		if t, err := time.Parse("2006-01-02", dateToStr); err == nil {
			end := t.Add(24*time.Hour - time.Second)
			dateTo = &end
		}
	}

	summary, err := h.auditService.GetActivitySummary(c.Context(), dateFrom, dateTo)
	if err != nil {
		return responses.InternalError(c, "Could not fetch activity summary")
	}

	result := make([]fiber.Map, len(summary))
	for i, s := range summary {
		result[i] = fiber.Map{
			"event_type": s.EventType,
			"count":      s.Count,
		}
	}

	return c.JSON(result)
}

// ExportCourseAuditLogCSV handles GET /courses/:course_id/audit_log.csv
func (h *AuditHandler) ExportCourseAuditLogCSV(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	filter := parseAuditLogFilter(c)
	cid := uint(courseID)
	filter.CourseID = &cid

	csvData, err := h.auditService.ExportAuditLogCSV(c.Context(), filter)
	if err != nil {
		return responses.InternalError(c, "Could not export audit log")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"audit_log.csv\"")
	return c.Send(csvData)
}

// ExportCourseGradeChangeLogCSV handles GET /courses/:course_id/grade_change_log.csv
func (h *AuditHandler) ExportCourseGradeChangeLogCSV(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	filter := parseGradeChangeLogFilter(c)
	cid := uint(courseID)
	filter.CourseID = &cid

	csvData, err := h.auditService.ExportGradeChangeLogCSV(c.Context(), filter)
	if err != nil {
		return responses.InternalError(c, "Could not export grade change log")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"grade_change_log.csv\"")
	return c.Send(csvData)
}
