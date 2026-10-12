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

func (r *calendarEventRepo) FindByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error) {
	var event models.CalendarEvent
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Context-polymorphic tenant scope:
		//  Course → context_id→courses.account_id
		//  Account → context_id IS account_id
		//  Group → context_id→groups (which is itself dual-scope)
		//  User → context_id→users.account_id
		q = q.Where(`
			(context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			OR (context_type = 'Account' AND context_id = ?)
			OR (context_type = 'User' AND context_id IN (SELECT id FROM users WHERE account_id = ?))
			OR (context_type = 'CourseSection' AND context_id IN (
				SELECT id FROM course_sections WHERE course_id IN (SELECT id FROM courses WHERE account_id = ?)
			))
			OR (context_type = 'Group' AND context_id IN (
				SELECT id FROM groups WHERE
					(context_type = 'Account' AND context_id = ?)
					OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
			))
		`, accountID, accountID, accountID, accountID, accountID, accountID)
	}
	if err := q.First(&event, id).Error; err != nil {
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

// ListBySeries returns every live occurrence of a recurring series in
// start order.
func (r *calendarEventRepo) ListBySeries(ctx context.Context, seriesUUID string) ([]models.CalendarEvent, error) {
	var events []models.CalendarEvent
	err := r.db.WithContext(ctx).
		Where("series_uuid = ? AND workflow_state != ?", seriesUUID, "deleted").
		Order("start_at ASC, id ASC").
		Find(&events).Error
	return events, err
}

// ListChildren returns the live section-specific child events of a
// course event.
func (r *calendarEventRepo) ListChildren(ctx context.Context, parentID uint) ([]models.CalendarEvent, error) {
	var events []models.CalendarEvent
	err := r.db.WithContext(ctx).
		Where("parent_calendar_event_id = ? AND workflow_state != ?", parentID, "deleted").
		Order("start_at ASC, id ASC").
		Find(&events).Error
	return events, err
}

// CreateMany inserts a batch of events in one transaction (a recurring
// series, or a parent's section children).
func (r *calendarEventRepo) CreateMany(ctx context.Context, events []*models.CalendarEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, e := range events {
			if err := tx.Create(e).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveMany updates a batch of events in one transaction.
func (r *calendarEventRepo) SaveMany(ctx context.Context, events []*models.CalendarEvent) error {
	if len(events) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, e := range events {
			if err := tx.Save(e).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMany soft-deletes the given events.
func (r *calendarEventRepo) DeleteMany(ctx context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&models.CalendarEvent{}).Where("id IN ?", ids).Update("workflow_state", "deleted").Error
}
