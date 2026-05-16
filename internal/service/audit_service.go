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

// AuditService provides business logic for audit logging and querying.
type AuditService struct {
	auditLogRepo       *postgres.AuditLogRepo
	gradeChangeLogRepo *postgres.GradeChangeLogRepo
	// 13.5 — PII access log repo is wired here so handlers reach a
	// single audit surface (`auditService`) rather than having to
	// thread FERPAService through everywhere they read student data.
	piiLogRepo postgres.PIIAccessLogRepository
}

// NewAuditService creates a new AuditService with the given repository dependencies.
// piiLogRepo may be nil — `LogPIIAccess` becomes a no-op when not wired (useful for tests
// that don't exercise the FERPA surface).
func NewAuditService(auditLogRepo *postgres.AuditLogRepo, gradeChangeLogRepo *postgres.GradeChangeLogRepo, piiLogRepo postgres.PIIAccessLogRepository) *AuditService {
	return &AuditService{
		auditLogRepo:       auditLogRepo,
		gradeChangeLogRepo: gradeChangeLogRepo,
		piiLogRepo:         piiLogRepo,
	}
}

// LogPIIAccess creates a PII access log entry for FERPA audit trail.
// Same shape as FERPAService.LogPIIAccess but reachable from handlers that
// only hold an *AuditService. Returns nil if the PII log repo is unwired
// (test wiring) — fire-and-forget callers shouldn't 5xx on audit failure.
func (s *AuditService) LogPIIAccess(ctx context.Context, accessorID, studentID uint, accessType, dataField, resource string, resourceID uint, ipAddress, userAgent string) error {
	if s == nil || s.piiLogRepo == nil {
		return nil
	}
	if accessorID == 0 {
		return errors.New("accessor_id is required")
	}
	// studentID == 0 is permitted for bulk-read summaries (e.g., a teacher
	// pulling the whole gradebook): the handler emits one row with
	// student_id=0 and data_field="bulk_<resource>_read" rather than
	// N rows. Documented at the call site.
	log := &models.PIIAccessLog{
		AccessorID: accessorID,
		StudentID:  studentID,
		AccessType: accessType,
		DataField:  dataField,
		Resource:   resource,
		ResourceID: resourceID,
		IPAddress:  ipAddress,
		UserAgent:  userAgent,
	}
	return s.piiLogRepo.Create(ctx, log)
}

// LogEvent creates an audit log entry for a system event.
// No-ops when the service or auditLogRepo is nil so test wiring that
// only exercises the PII surface can leave auditLogRepo unwired.
func (s *AuditService) LogEvent(ctx context.Context, eventType string, userID uint, courseID, accountID *uint, contextType string, contextID uint, action, payload, ipAddress, userAgent string) error {
	if s == nil || s.auditLogRepo == nil {
		return nil
	}
	log := &models.AuditLog{
		EventType:   eventType,
		UserID:      userID,
		CourseID:    courseID,
		AccountID:   accountID,
		ContextType: contextType,
		ContextID:   contextID,
		Action:      action,
		Payload:     payload,
		IPAddress:   ipAddress,
		UserAgent:   userAgent,
	}
	return s.auditLogRepo.Create(ctx, log)
}

// LogGradeChange creates both an AuditLog entry (event_type=grade_change) and a GradeChangeLog entry.
func (s *AuditService) LogGradeChange(ctx context.Context, courseID, assignmentID, studentID, graderID, submissionID uint, oldGrade, newGrade string, oldScore, newScore *float64, excused bool, gradingMethod string) error {
	// Create the grade change log entry
	gradeLog := &models.GradeChangeLog{
		CourseID:      courseID,
		AssignmentID:  assignmentID,
		StudentID:     studentID,
		GraderID:      graderID,
		SubmissionID:  submissionID,
		OldGrade:      oldGrade,
		NewGrade:      newGrade,
		OldScore:      oldScore,
		NewScore:      newScore,
		Excused:       excused,
		GradingMethod: gradingMethod,
	}

	if err := s.gradeChangeLogRepo.Create(ctx, gradeLog); err != nil {
		return err
	}

	// Also create an audit log entry for the grade change
	oldScoreStr := ""
	newScoreStr := ""
	if oldScore != nil {
		oldScoreStr = fmt.Sprintf("%.2f", *oldScore)
	}
	if newScore != nil {
		newScoreStr = fmt.Sprintf("%.2f", *newScore)
	}

	payload := fmt.Sprintf(`{"old_grade":"%s","new_grade":"%s","old_score":"%s","new_score":"%s","grading_method":"%s","excused":%t}`,
		oldGrade, newGrade, oldScoreStr, newScoreStr, gradingMethod, excused)

	auditLog := &models.AuditLog{
		EventType:   "grade_change",
		UserID:      graderID,
		CourseID:    &courseID,
		ContextType: "Submission",
		ContextID:   submissionID,
		Action:      "graded",
		Payload:     payload,
	}

	return s.auditLogRepo.Create(ctx, auditLog)
}

// GetAuditLog returns a paginated, filtered list of audit log entries.
func (s *AuditService) GetAuditLog(ctx context.Context, filter postgres.AuditLogFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	return s.auditLogRepo.ListByFilter(ctx, filter, params)
}

// GetGradeChangeLog returns a paginated, filtered list of grade change log entries.
func (s *AuditService) GetGradeChangeLog(ctx context.Context, filter postgres.GradeChangeLogFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	return s.gradeChangeLogRepo.ListByFilter(ctx, filter, params)
}

// GetCourseAuditLog returns all audit log entries for a specific course.
func (s *AuditService) GetCourseAuditLog(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	return s.auditLogRepo.ListByCourseID(ctx, courseID, params)
}

// GetActivitySummary returns event type counts within a date range for the admin dashboard.
func (s *AuditService) GetActivitySummary(ctx context.Context, dateFrom, dateTo *time.Time) ([]postgres.EventTypeCount, error) {
	return s.auditLogRepo.CountByEventType(ctx, dateFrom, dateTo)
}

// ExportAuditLogCSV exports audit log entries matching the filter as CSV bytes.
func (s *AuditService) ExportAuditLogCSV(ctx context.Context, filter postgres.AuditLogFilter) ([]byte, error) {
	logs, err := s.auditLogRepo.ListAllByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Event Type", "User ID", "Course ID", "Account ID", "Context Type", "Context ID", "Action", "Payload", "IP Address", "User Agent", "Created At"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write rows
	for _, log := range logs {
		courseID := ""
		if log.CourseID != nil {
			courseID = fmt.Sprintf("%d", *log.CourseID)
		}
		accountID := ""
		if log.AccountID != nil {
			accountID = fmt.Sprintf("%d", *log.AccountID)
		}

		row := []string{
			fmt.Sprintf("%d", log.ID),
			log.EventType,
			fmt.Sprintf("%d", log.UserID),
			courseID,
			accountID,
			log.ContextType,
			fmt.Sprintf("%d", log.ContextID),
			log.Action,
			log.Payload,
			log.IPAddress,
			log.UserAgent,
			log.CreatedAt.Format(time.RFC3339),
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

// ExportGradeChangeLogCSV exports grade change log entries matching the filter as CSV bytes for compliance reporting.
func (s *AuditService) ExportGradeChangeLogCSV(ctx context.Context, filter postgres.GradeChangeLogFilter) ([]byte, error) {
	logs, err := s.gradeChangeLogRepo.ListAllByFilter(ctx, filter)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"ID", "Course ID", "Assignment ID", "Student ID", "Grader ID", "Submission ID", "Old Grade", "New Grade", "Old Score", "New Score", "Excused", "Grading Method", "Created At"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write rows
	for _, log := range logs {
		oldScore := ""
		if log.OldScore != nil {
			oldScore = fmt.Sprintf("%.2f", *log.OldScore)
		}
		newScore := ""
		if log.NewScore != nil {
			newScore = fmt.Sprintf("%.2f", *log.NewScore)
		}

		row := []string{
			fmt.Sprintf("%d", log.ID),
			fmt.Sprintf("%d", log.CourseID),
			fmt.Sprintf("%d", log.AssignmentID),
			fmt.Sprintf("%d", log.StudentID),
			fmt.Sprintf("%d", log.GraderID),
			fmt.Sprintf("%d", log.SubmissionID),
			log.OldGrade,
			log.NewGrade,
			oldScore,
			newScore,
			fmt.Sprintf("%t", log.Excused),
			log.GradingMethod,
			log.CreatedAt.Format(time.RFC3339),
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
