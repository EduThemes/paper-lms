package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// GradeChangeLogFilter defines the filtering criteria for querying grade change logs.
type GradeChangeLogFilter struct {
	CourseID     *uint
	StudentID    *uint
	GraderID     *uint
	AssignmentID *uint
	DateFrom     *time.Time
	DateTo       *time.Time
}

// GradeChangeLogRepo implements grade change log persistence with PostgreSQL.
type GradeChangeLogRepo struct {
	db *gorm.DB
}

// NewGradeChangeLogRepository creates a new grade change log repository backed by PostgreSQL.
func NewGradeChangeLogRepository(db *gorm.DB) *GradeChangeLogRepo {
	return &GradeChangeLogRepo{db: db}
}

// Create inserts a new grade change log entry.
func (r *GradeChangeLogRepo) Create(ctx context.Context, log *models.GradeChangeLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListByCourseID returns a paginated list of grade change logs for a specific course.
func (r *GradeChangeLogRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	return r.ListByFilter(ctx, GradeChangeLogFilter{CourseID: &courseID}, params)
}

// ListByStudentID returns a paginated list of grade change logs for a specific student.
func (r *GradeChangeLogRepo) ListByStudentID(ctx context.Context, studentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	return r.ListByFilter(ctx, GradeChangeLogFilter{StudentID: &studentID}, params)
}

// ListByGraderID returns a paginated list of grade change logs for a specific grader.
func (r *GradeChangeLogRepo) ListByGraderID(ctx context.Context, graderID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	return r.ListByFilter(ctx, GradeChangeLogFilter{GraderID: &graderID}, params)
}

// ListByAssignmentID returns a paginated list of grade change logs for a specific assignment.
func (r *GradeChangeLogRepo) ListByAssignmentID(ctx context.Context, assignmentID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	return r.ListByFilter(ctx, GradeChangeLogFilter{AssignmentID: &assignmentID}, params)
}

// ListByFilter returns a paginated list of grade change logs matching the given filter.
func (r *GradeChangeLogRepo) ListByFilter(ctx context.Context, filter GradeChangeLogFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.GradeChangeLog], error) {
	var logs []models.GradeChangeLog
	var count int64

	query := r.db.WithContext(ctx).Model(&models.GradeChangeLog{})
	query = applyGradeChangeLogFilter(query, filter)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GradeChangeLog]{
		Items:      logs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// ListAllByFilter returns all grade change logs matching the filter (unpaginated, for CSV export).
func (r *GradeChangeLogRepo) ListAllByFilter(ctx context.Context, filter GradeChangeLogFilter) ([]models.GradeChangeLog, error) {
	var logs []models.GradeChangeLog

	query := r.db.WithContext(ctx).Model(&models.GradeChangeLog{})
	query = applyGradeChangeLogFilter(query, filter)

	if err := query.Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// applyGradeChangeLogFilter applies optional filter criteria to a GORM query.
func applyGradeChangeLogFilter(query *gorm.DB, filter GradeChangeLogFilter) *gorm.DB {
	if filter.CourseID != nil {
		query = query.Where("course_id = ?", *filter.CourseID)
	}
	if filter.StudentID != nil {
		query = query.Where("student_id = ?", *filter.StudentID)
	}
	if filter.GraderID != nil {
		query = query.Where("grader_id = ?", *filter.GraderID)
	}
	if filter.AssignmentID != nil {
		query = query.Where("assignment_id = ?", *filter.AssignmentID)
	}
	if filter.DateFrom != nil {
		query = query.Where("created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("created_at <= ?", *filter.DateTo)
	}
	return query
}
