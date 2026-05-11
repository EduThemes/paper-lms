package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/api/v1/responses"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
)

type AttendanceHandler struct {
	attendanceService *service.AttendanceService
}

func NewAttendanceHandler(attendanceService *service.AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{attendanceService: attendanceService}
}

func attendanceRecordToJSON(r *models.AttendanceRecord) fiber.Map {
	return fiber.Map{
		"id":           r.ID,
		"course_id":    r.CourseID,
		"section_id":   r.SectionID,
		"user_id":      r.UserID,
		"date":         r.Date.Format("2006-01-02"),
		"status":       r.Status,
		"notes":        r.Notes,
		"marked_by_id": r.MarkedByID,
		"created_at":   r.CreatedAt,
		"updated_at":   r.UpdatedAt,
	}
}

// RecordAttendance handles POST /api/v1/courses/:course_id/attendance
// Supports both single and bulk attendance recording.
func (h *AttendanceHandler) RecordAttendance(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	// Try to parse as bulk first
	var bulkInput struct {
		Date    string `json:"date"`
		Records []struct {
			UserID     uint   `json:"user_id"`
			SectionID  *uint  `json:"section_id"`
			Status     string `json:"status"`
			Notes      string `json:"notes"`
			MarkedByID uint   `json:"marked_by_id"`
		} `json:"records"`
	}

	if err := c.BodyParser(&bulkInput); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	// Determine if this is a bulk or single request
	if len(bulkInput.Records) > 0 {
		// Bulk attendance
		date, err := time.Parse("2006-01-02", bulkInput.Date)
		if err != nil {
			return responses.BadRequest(c, "Invalid date format, expected YYYY-MM-DD")
		}

		records := make([]models.AttendanceRecord, len(bulkInput.Records))
		for i, r := range bulkInput.Records {
			records[i] = models.AttendanceRecord{
				CourseID:   uint(courseID),
				SectionID:  r.SectionID,
				UserID:     r.UserID,
				Date:       date,
				Status:     r.Status,
				Notes:      r.Notes,
				MarkedByID: r.MarkedByID,
			}
		}

		if err := h.attendanceService.BulkRecordAttendance(c.Context(), uint(courseID), date, records); err != nil {
			return responses.BadRequest(c, err.Error())
		}

		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"recorded": len(records),
			"date":     bulkInput.Date,
		})
	}

	// Single attendance record
	var singleInput struct {
		Attendance struct {
			UserID     uint   `json:"user_id"`
			SectionID  *uint  `json:"section_id"`
			Date       string `json:"date"`
			Status     string `json:"status"`
			Notes      string `json:"notes"`
			MarkedByID uint   `json:"marked_by_id"`
		} `json:"attendance"`
	}

	if err := c.BodyParser(&singleInput); err != nil {
		return responses.BadRequest(c, "Invalid input")
	}

	date, err := time.Parse("2006-01-02", singleInput.Attendance.Date)
	if err != nil {
		return responses.BadRequest(c, "Invalid date format, expected YYYY-MM-DD")
	}

	record := &models.AttendanceRecord{
		CourseID:   uint(courseID),
		SectionID:  singleInput.Attendance.SectionID,
		UserID:     singleInput.Attendance.UserID,
		Date:       date,
		Status:     singleInput.Attendance.Status,
		Notes:      singleInput.Attendance.Notes,
		MarkedByID: singleInput.Attendance.MarkedByID,
	}

	if err := h.attendanceService.RecordAttendance(c.Context(), record); err != nil {
		return responses.BadRequest(c, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(attendanceRecordToJSON(record))
}

// GetClassAttendance handles GET /api/v1/courses/:course_id/attendance?date=YYYY-MM-DD
func (h *AttendanceHandler) GetClassAttendance(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return responses.BadRequest(c, "Invalid date format, expected YYYY-MM-DD")
	}

	params := middleware.GetPagination(c)

	result, err := h.attendanceService.GetAttendanceForDate(c.Context(), uint(courseID), date, params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch attendance records")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		items[i] = attendanceRecordToJSON(&r)
	}

	return c.JSON(items)
}

// GetStudentAttendance handles GET /api/v1/courses/:course_id/users/:user_id/attendance
func (h *AttendanceHandler) GetStudentAttendance(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	params := middleware.GetPagination(c)

	result, err := h.attendanceService.GetStudentAttendance(c.Context(), uint(userID), uint(courseID), params)
	if err != nil {
		return responses.InternalError(c, "Could not fetch attendance records")
	}

	responses.SetPaginationHeaders(c, result.TotalCount, result.Page, result.PerPage)

	items := make([]fiber.Map, len(result.Items))
	for i, r := range result.Items {
		items[i] = attendanceRecordToJSON(&r)
	}

	return c.JSON(items)
}

// GetStudentAttendanceSummary handles GET /api/v1/courses/:course_id/users/:user_id/attendance/summary
func (h *AttendanceHandler) GetStudentAttendanceSummary(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	userID, err := c.ParamsInt("user_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid user ID")
	}

	summary, err := h.attendanceService.GetAttendanceSummary(c.Context(), uint(userID), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not calculate attendance summary")
	}

	return c.JSON(fiber.Map{
		"user_id":         summary.UserID,
		"course_id":       summary.CourseID,
		"total_days":      summary.TotalDays,
		"present_days":    summary.PresentDays,
		"absent_days":     summary.AbsentDays,
		"tardy_days":      summary.TardyDays,
		"excused_days":    summary.ExcusedDays,
		"attendance_rate": summary.AttendanceRate,
	})
}

// ExportAttendanceCSV handles GET /api/v1/courses/:course_id/attendance/export
func (h *AttendanceHandler) ExportAttendanceCSV(c *fiber.Ctx) error {
	courseID, err := c.ParamsInt("course_id")
	if err != nil {
		return responses.BadRequest(c, "Invalid course ID")
	}

	csvData, err := h.attendanceService.ExportAttendanceCSV(c.Context(), uint(courseID))
	if err != nil {
		return responses.InternalError(c, "Could not export attendance")
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"attendance.csv\"")
	return c.Send(csvData)
}
