package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type learningOutcomeGroupRepo struct {
	db *gorm.DB
}

func NewLearningOutcomeGroupRepository(db *gorm.DB) repository.LearningOutcomeGroupRepository {
	return &learningOutcomeGroupRepo{db: db}
}

func (r *learningOutcomeGroupRepo) Create(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *learningOutcomeGroupRepo) FindByID(ctx context.Context, id uint) (*models.LearningOutcomeGroup, error) {
	var group models.LearningOutcomeGroup
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *learningOutcomeGroupRepo) Update(ctx context.Context, group *models.LearningOutcomeGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *learningOutcomeGroupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.LearningOutcomeGroup{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *learningOutcomeGroupRepo) ListByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	var groups []models.LearningOutcomeGroup
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcomeGroup{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.LearningOutcomeGroup]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *learningOutcomeGroupRepo) FindRootGroup(ctx context.Context, contextType string, contextID uint) (*models.LearningOutcomeGroup, error) {
	var group models.LearningOutcomeGroup
	if err := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND parent_group_id IS NULL AND workflow_state != ?", contextType, contextID, "deleted").First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
