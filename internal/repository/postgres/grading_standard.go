package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type gradingStandardRepo struct {
	db *gorm.DB
}

func NewGradingStandardRepository(db *gorm.DB) repository.GradingStandardRepository {
	return &gradingStandardRepo{db: db}
}

func (r *gradingStandardRepo) Create(ctx context.Context, standard *models.GradingStandard) error {
	return r.db.WithContext(ctx).Create(standard).Error
}

func (r *gradingStandardRepo) FindByID(ctx context.Context, id uint) (*models.GradingStandard, error) {
	var standard models.GradingStandard
	if err := r.db.WithContext(ctx).First(&standard, id).Error; err != nil {
		return nil, err
	}
	return &standard, nil
}

func (r *gradingStandardRepo) Update(ctx context.Context, standard *models.GradingStandard) error {
	return r.db.WithContext(ctx).Save(standard).Error
}

func (r *gradingStandardRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.GradingStandard{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *gradingStandardRepo) ListByCourse(ctx context.Context, courseID uint) ([]models.GradingStandard, error) {
	var standards []models.GradingStandard
	if err := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND workflow_state = ?", "Course", courseID, "active").Order("id ASC").Find(&standards).Error; err != nil {
		return nil, err
	}
	return standards, nil
}

func (r *gradingStandardRepo) FindActiveByCourse(ctx context.Context, courseID uint) (*models.GradingStandard, error) {
	var standard models.GradingStandard
	if err := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND workflow_state = ?", "Course", courseID, "active").Order("id DESC").First(&standard).Error; err != nil {
		return nil, err
	}
	return &standard, nil
}
