package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// GamificationEventRepo persists xAPI-shaped events for Phase 6 gamification.
type GamificationEventRepo struct {
	db *gorm.DB
}

// NewGamificationEventRepository constructs a GamificationEventRepo.
func NewGamificationEventRepository(db *gorm.DB) *GamificationEventRepo {
	return &GamificationEventRepo{db: db}
}

func (r *GamificationEventRepo) Create(ctx context.Context, event *models.GamificationEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *GamificationEventRepo) FindByID(ctx context.Context, id uint) (*models.GamificationEvent, error) {
	var event models.GamificationEvent
	if err := r.db.WithContext(ctx).First(&event, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (r *GamificationEventRepo) FindBySourceEventID(ctx context.Context, source, sourceEventID string) (*models.GamificationEvent, error) {
	var event models.GamificationEvent
	err := r.db.WithContext(ctx).
		Where("source = ? AND source_event_id = ?", source, sourceEventID).
		First(&event).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &event, nil
}

func (r *GamificationEventRepo) List(ctx context.Context, filter repository.GamificationEventFilter, params repository.PaginationParams) (*repository.PaginatedResult[models.GamificationEvent], error) {
	query := r.db.WithContext(ctx).Model(&models.GamificationEvent{})

	if filter.TenantID != nil {
		query = query.Where("tenant_id = ?", *filter.TenantID)
	}
	if filter.ActorID != nil {
		query = query.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.Verb != "" {
		query = query.Where("verb = ?", filter.Verb)
	}
	if filter.ObjectType != "" {
		query = query.Where("object_type = ?", filter.ObjectType)
	}
	if filter.ObjectID != nil {
		query = query.Where("object_id = ?", *filter.ObjectID)
	}
	if filter.OccurredFrom != nil {
		query = query.Where("occurred_at >= ?", *filter.OccurredFrom)
	}
	if filter.OccurredTo != nil {
		query = query.Where("occurred_at <= ?", *filter.OccurredTo)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return nil, err
	}

	var events []models.GamificationEvent
	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("occurred_at DESC").Find(&events).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GamificationEvent]{
		Items:      events,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
