package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conferenceRepo struct {
	db *gorm.DB
}

func NewConferenceRepository(db *gorm.DB) repository.ConferenceRepository {
	return &conferenceRepo{db: db}
}

func (r *conferenceRepo) Create(ctx context.Context, conference *models.Conference) error {
	return r.db.WithContext(ctx).Create(conference).Error
}

func (r *conferenceRepo) FindByID(ctx context.Context, id uint) (*models.Conference, error) {
	var conference models.Conference
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&conference, id).Error; err != nil {
		return nil, err
	}
	return &conference, nil
}

func (r *conferenceRepo) Update(ctx context.Context, conference *models.Conference) error {
	return r.db.WithContext(ctx).Save(conference).Error
}

func (r *conferenceRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Conference{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *conferenceRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	var conferences []models.Conference
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Conference{}).
		Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&conferences).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Conference]{
		Items:      conferences,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
