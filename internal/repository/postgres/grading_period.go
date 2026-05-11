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

func (r *gradingPeriodRepo) FindByID(ctx context.Context, id uint) (*models.GradingPeriod, error) {
	var period models.GradingPeriod
	if err := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted").First(&period).Error; err != nil {
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

func (r *gradingPeriodRepo) ListByGroupID(ctx context.Context, groupID uint) ([]models.GradingPeriod, error) {
	var periods []models.GradingPeriod
	if err := r.db.WithContext(ctx).Where("grading_period_group_id = ? AND workflow_state != ?", groupID, "deleted").Order("start_date ASC").Find(&periods).Error; err != nil {
		return nil, err
	}
	return periods, nil
}
