package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type calendarEventRepo struct {
	db *gorm.DB
}

func NewCalendarEventRepository(db *gorm.DB) repository.CalendarEventRepository {
	return &calendarEventRepo{db: db}
}

func (r *calendarEventRepo) Create(ctx context.Context, event *models.CalendarEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *calendarEventRepo) FindByID(ctx context.Context, id uint) (*models.CalendarEvent, error) {
	var event models.CalendarEvent
	if err := r.db.WithContext(ctx).First(&event, id).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *calendarEventRepo) Update(ctx context.Context, event *models.CalendarEvent) error {
	return r.db.WithContext(ctx).Save(event).Error
}

func (r *calendarEventRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.CalendarEvent{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *calendarEventRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CalendarEvent], error) {
	var events []models.CalendarEvent
	var count int64

	query := r.db.WithContext(ctx).Model(&models.CalendarEvent{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("start_at ASC").Find(&events).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.CalendarEvent]{
		Items:      events,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *calendarEventRepo) ListByContextAndDateRange(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time) ([]models.CalendarEvent, error) {
	var events []models.CalendarEvent
	if err := r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ? AND workflow_state != ? AND start_at >= ? AND start_at <= ?", contextType, contextID, "deleted", startAt, endAt).
		Order("start_at ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}
