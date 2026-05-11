package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type collaborationRepo struct {
	db *gorm.DB
}

func NewCollaborationRepository(db *gorm.DB) repository.CollaborationRepository {
	return &collaborationRepo{db: db}
}

func (r *collaborationRepo) Create(ctx context.Context, collaboration *models.Collaboration) error {
	return r.db.WithContext(ctx).Create(collaboration).Error
}

func (r *collaborationRepo) FindByID(ctx context.Context, id uint) (*models.Collaboration, error) {
	var collaboration models.Collaboration
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&collaboration, id).Error; err != nil {
		return nil, err
	}
	return &collaboration, nil
}

func (r *collaborationRepo) Update(ctx context.Context, collaboration *models.Collaboration) error {
	return r.db.WithContext(ctx).Save(collaboration).Error
}

func (r *collaborationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Collaboration{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *collaborationRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Collaboration], error) {
	var collaborations []models.Collaboration
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Collaboration{}).
		Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&collaborations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Collaboration]{
		Items:      collaborations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
