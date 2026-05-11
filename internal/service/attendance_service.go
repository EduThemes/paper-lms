package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
)

// AttendanceService provides business logic for attendance tracking.
type AttendanceService struct {
	repo postgres.AttendanceRepository
}

// NewAttendanceService creates a new AttendanceService with the given repository dependency.
func NewAttendanceService(repo postgres.AttendanceRepository) *AttendanceService {
	return &AttendanceService{repo: repo}
}

// RecordAttendance creates or updates an attendance record for a specific date.
// If a record already exists for the same course/user/date, it updates the existing one.
func (s *AttendanceService) RecordAttendance(ctx context.Context, record *models.AttendanceRecord) error {
	if record.CourseID == 0 {
		return errors.New("course_id is required")
	}
	if record.UserID == 0 {
		return errors.New("user_id is required")
	}
	if record.Date.IsZero() {
		return errors.New("date is required")
	}
	if !isValidAttendanceStatus(record.Status) {
		return errors.New("status must be one of: present, absent, tardy, excused")
	}
	if record.MarkedByID == 0 {
		return errors.New("marked_by_id is required")
	}

	// Check if record already exists for this course/user/date
	existing, _ := s.repo.FindByCourseUserDate(ctx, record.CourseID, record.UserID, record.Date)
	if existing != nil {
		existing.Status = record.Status
		existing.Notes = record.Notes
		existing.MarkedByID = record.MarkedByID
		return s.repo.Update(ctx, existing)
	}

	return s.repo.Create(ctx, record)
}

// BulkRecordAttendance marks attendance for multiple students at once for a given course and date.
func (s *AttendanceService) BulkRecordAttendance(ctx context.Context, courseID uint, date time.Time, records []models.AttendanceRecord) error {
	if courseID == 0 {
		return errors.New("course_id is required")
	}
	if date.IsZero() {
		return errors.New("date is required")
	}

	// Normalize all records with the given courseID and date
	for i := range records {
		records[i].CourseID = courseID
		records[i].Date = date
		if !isValidAttendanceStatus(records[i].Status) {
			return fmt.Errorf("invalid status '%s' for user_id %d", records[i].Status, records[i].UserID)
		}
	}

	return s.repo.BulkCreate(ctx, records)
}

// GetAttendanceForDate retrieves all attendance records for a course on a specific date.
func (s *AttendanceService) GetAttendanceForDate(ctx context.Context, courseID uint, date time.Time, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error) {
	return s.repo.ListByCourseAndDate(ctx, courseID, date, params)
}

// GetStudentAttendance retrieves a student's attendance history for a specific course.
func (s *AttendanceService) GetStudentAttendance(ctx context.Context, userID uint, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AttendanceRecord], error) {
	return s.repo.ListByUserAndCourse(ctx, userID, courseID, params)
}

// GetAttendanceSummary calculates attendance summary statistics for a student in a course.
func (s *AttendanceService) GetAttendanceSummary(ctx context.Context, userID uint, courseID uint) (*models.AttendanceSummary, error) {
	return s.repo.GetSummary(ctx, userID, courseID)
}

// ExportAttendanceCSV generates a CSV export of all attendance records for a course.
func (s *AttendanceService) ExportAttendanceCSV(ctx context.Context, courseID uint) ([]byte, error) {
	params := repository.PaginationParams{Page: 1, PerPage: 10000}
	var allRecords []models.AttendanceRecord

	for {
		result, err := s.repo.ListByCourse(ctx, courseID, params)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch attendance records: %w", err)
		}
		allRecords = append(allRecords, result.Items...)
		if int64(len(allRecords)) >= result.TotalCount {
			break
		}
		params.Page++
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"ID", "Course ID", "Section ID", "User ID", "Date", "Status", "Notes", "Marked By", "Created At"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, record := range allRecords {
		sectionID := ""
		if record.SectionID != nil {
			sectionID = fmt.Sprintf("%d", *record.SectionID)
		}

		row := []string{
			fmt.Sprintf("%d", record.ID),
			fmt.Sprintf("%d", record.CourseID),
			sectionID,
			fmt.Sprintf("%d", record.UserID),
			record.Date.Format("2006-01-02"),
			record.Status,
			record.Notes,
			fmt.Sprintf("%d", record.MarkedByID),
			record.CreatedAt.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func isValidAttendanceStatus(status string) bool {
	switch status {
	case "present", "absent", "tardy", "excused":
		return true
	default:
		return false
	}
}
