package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type gradingPeriodGroupRepo struct {
	db *gorm.DB
}

func NewGradingPeriodGroupRepository(db *gorm.DB) repository.GradingPeriodGroupRepository {
	return &gradingPeriodGroupRepo{db: db}
}

func (r *gradingPeriodGroupRepo) Create(ctx context.Context, group *models.GradingPeriodGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *gradingPeriodGroupRepo) FindByID(ctx context.Context, id uint) (*models.GradingPeriodGroup, error) {
	var group models.GradingPeriodGroup
	if err := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted").First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *gradingPeriodGroupRepo) Update(ctx context.Context, group *models.GradingPeriodGroup) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *gradingPeriodGroupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.GradingPeriodGroup{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *gradingPeriodGroupRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradingPeriodGroup], error) {
	var groups []models.GradingPeriodGroup
	var count int64

	query := r.db.WithContext(ctx).Model(&models.GradingPeriodGroup{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GradingPeriodGroup]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
