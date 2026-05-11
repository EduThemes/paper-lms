package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// AuditLogFilter defines the filtering criteria for querying audit logs.
type AuditLogFilter struct {
	EventType   string
	UserID      *uint
	CourseID    *uint
	AccountID   *uint
	ContextType string
	DateFrom    *time.Time
	DateTo      *time.Time
}

// EventTypeCount holds the count of audit log entries grouped by event type.
type EventTypeCount struct {
	EventType string `json:"event_type"`
	Count     int64  `json:"count"`
}

// AuditLogRepo implements audit log persistence with PostgreSQL.
type AuditLogRepo struct {
	db *gorm.DB
}

// NewAuditLogRepository creates a new audit log repository backed by PostgreSQL.
func NewAuditLogRepository(db *gorm.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

// Create inserts a new audit log entry.
func (r *AuditLogRepo) Create(ctx context.Context, log *models.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// ListByFilter returns a paginated list of audit logs matching the given filter.
func (r *AuditLogRepo) ListByFilter(ctx context.Context, filter AuditLogFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	var logs []models.AuditLog
	var count int64

	query := r.db.WithContext(ctx).Model(&models.AuditLog{})
	query = applyAuditLogFilter(query, filter)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AuditLog]{
		Items:      logs,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

// CountByEventType returns aggregated counts per event type within a date range.
func (r *AuditLogRepo) CountByEventType(ctx context.Context, dateFrom, dateTo *time.Time) ([]EventTypeCount, error) {
	query := r.db.WithContext(ctx).Model(&models.AuditLog{}).
		Select("event_type, COUNT(*) as count")

	if dateFrom != nil {
		query = query.Where("created_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		query = query.Where("created_at <= ?", *dateTo)
	}

	query = query.Group("event_type").Order("count DESC")

	rows, err := query.Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []EventTypeCount
	for rows.Next() {
		var etc EventTypeCount
		if err := rows.Scan(&etc.EventType, &etc.Count); err != nil {
			return nil, err
		}
		results = append(results, etc)
	}

	if results == nil {
		results = []EventTypeCount{}
	}

	return results, nil
}

// ListByCourseID returns a paginated list of audit logs for a specific course.
func (r *AuditLogRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	return r.ListByFilter(ctx, AuditLogFilter{CourseID: &courseID}, params)
}

// ListByUserID returns a paginated list of audit logs for a specific user.
func (r *AuditLogRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	return r.ListByFilter(ctx, AuditLogFilter{UserID: &userID}, params)
}

// ListAll returns a paginated list of all audit logs without filtering.
func (r *AuditLogRepo) ListAll(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.AuditLog], error) {
	return r.ListByFilter(ctx, AuditLogFilter{}, params)
}

// ListAllByFilter returns all audit logs matching the filter (unpaginated, for CSV export).
func (r *AuditLogRepo) ListAllByFilter(ctx context.Context, filter AuditLogFilter) ([]models.AuditLog, error) {
	var logs []models.AuditLog

	query := r.db.WithContext(ctx).Model(&models.AuditLog{})
	query = applyAuditLogFilter(query, filter)

	if err := query.Order("created_at DESC").Find(&logs).Error; err != nil {
		return nil, err
	}

	return logs, nil
}

// applyAuditLogFilter applies optional filter criteria to a GORM query.
func applyAuditLogFilter(query *gorm.DB, filter AuditLogFilter) *gorm.DB {
	if filter.EventType != "" {
		query = query.Where("event_type = ?", filter.EventType)
	}
	if filter.UserID != nil {
		query = query.Where("user_id = ?", *filter.UserID)
	}
	if filter.CourseID != nil {
		query = query.Where("course_id = ?", *filter.CourseID)
	}
	if filter.AccountID != nil {
		query = query.Where("account_id = ?", *filter.AccountID)
	}
	if filter.ContextType != "" {
		query = query.Where("context_type = ?", filter.ContextType)
	}
	if filter.DateFrom != nil {
		query = query.Where("created_at >= ?", *filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("created_at <= ?", *filter.DateTo)
	}
	return query
}
