package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type gradingPeriodRepo struct {
	db *gorm.DB
}

func NewGradingPeriodRepository(db *gorm.DB) repository.GradingPeriodRepository {
	return &gradingPeriodRepo{db: db}
}

func (r *gradingPeriodRepo) Create(ctx context.Context, period *models.GradingPeriod) error {
	return r.db.WithContext(ctx).Create(period).Error
}

func (r *gradingPeriodRepo) FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriod, error) {
	var period models.GradingPeriod
	q := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted")
	if accountID != 0 {
		// Scope through grading_period_groups.account_id.
		q = q.Where("grading_period_group_id IN (SELECT id FROM grading_period_groups WHERE account_id = ?)", accountID)
	}
	if err := q.First(&period).Error; err != nil {
		return nil, err
	}
	return &period, nil
}

func (r *gradingPeriodRepo) Update(ctx context.Context, period *models.GradingPeriod) error {
	return r.db.WithContext(ctx).Save(period).Error
}

func (r *gradingPeriodRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.GradingPeriod{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *gradingPeriodRepo) ListByGroupID(ctx context.Context, groupID, accountID uint) ([]models.GradingPeriod, error) {
	var periods []models.GradingPeriod
	q := r.db.WithContext(ctx).Where("grading_period_group_id = ? AND workflow_state != ?", groupID, "deleted")
	if accountID != 0 {
		q = q.Where("grading_period_group_id IN (SELECT id FROM grading_period_groups WHERE account_id = ?)", accountID)
	}
	if err := q.Order("start_date ASC").Find(&periods).Error; err != nil {
		return nil, err
	}
	return periods, nil
}
