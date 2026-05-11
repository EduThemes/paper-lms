package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type learningOutcomeRepo struct {
	db *gorm.DB
}

func NewLearningOutcomeRepository(db *gorm.DB) repository.LearningOutcomeRepository {
	return &learningOutcomeRepo{db: db}
}

func (r *learningOutcomeRepo) Create(ctx context.Context, outcome *models.LearningOutcome) error {
	return r.db.WithContext(ctx).Create(outcome).Error
}

func (r *learningOutcomeRepo) FindByID(ctx context.Context, id uint) (*models.LearningOutcome, error) {
	var outcome models.LearningOutcome
	if err := r.db.WithContext(ctx).First(&outcome, id).Error; err != nil {
		return nil, err
	}
	return &outcome, nil
}

func (r *learningOutcomeRepo) Update(ctx context.Context, outcome *models.LearningOutcome) error {
	return r.db.WithContext(ctx).Save(outcome).Error
}

func (r *learningOutcomeRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *learningOutcomeRepo) ListByGroupID(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	var outcomes []models.LearningOutcome
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("outcome_group_id = ? AND workflow_state != ?", groupID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&outcomes).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcome]{
		Items:      outcomes,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *learningOutcomeRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcome], error) {
	var outcomes []models.LearningOutcome
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcome{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&outcomes).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcome]{
		Items:      outcomes,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
