package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type rubricRepo struct {
	db *gorm.DB
}

func NewRubricRepository(db *gorm.DB) repository.RubricRepository {
	return &rubricRepo{db: db}
}

func (r *rubricRepo) Create(ctx context.Context, rubric *models.Rubric) error {
	return r.db.WithContext(ctx).Create(rubric).Error
}

func (r *rubricRepo) FindByID(ctx context.Context, id uint) (*models.Rubric, error) {
	var rubric models.Rubric
	if err := r.db.WithContext(ctx).First(&rubric, id).Error; err != nil {
		return nil, err
	}
	return &rubric, nil
}

func (r *rubricRepo) Update(ctx context.Context, rubric *models.Rubric) error {
	return r.db.WithContext(ctx).Save(rubric).Error
}

func (r *rubricRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Rubric{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *rubricRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Rubric], error) {
	var rubrics []models.Rubric
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Rubric{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&rubrics).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Rubric]{
		Items:      rubrics,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
