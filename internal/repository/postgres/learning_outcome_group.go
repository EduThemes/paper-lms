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

func (r *learningOutcomeGroupRepo) FindByID(ctx context.Context, id, accountID uint) (*models.LearningOutcomeGroup, error) {
	var group models.LearningOutcomeGroup
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(`
			(context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
		`, accountID, accountID)
	}
	if err := q.First(&group, id).Error; err != nil {
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

func (r *learningOutcomeGroupRepo) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LearningOutcomeGroup], error) {
	var groups []models.LearningOutcomeGroup
	var count int64

	query := r.db.WithContext(ctx).Model(&models.LearningOutcomeGroup{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	if accountID != 0 {
		switch contextType {
		case "Course":
			query = query.Where("context_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
		case "Account":
			query = query.Where("context_id = ?", accountID)
		}
	}
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

func (r *learningOutcomeGroupRepo) FindRootGroup(ctx context.Context, contextType string, contextID, accountID uint) (*models.LearningOutcomeGroup, error) {
	var group models.LearningOutcomeGroup
	q := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND parent_group_id IS NULL AND workflow_state != ?", contextType, contextID, "deleted")
	if accountID != 0 {
		switch contextType {
		case "Course":
			q = q.Where("context_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
		case "Account":
			q = q.Where("context_id = ?", accountID)
		}
	}
	if err := q.First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
