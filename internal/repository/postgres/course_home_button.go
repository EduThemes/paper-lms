package postgres

import (
	"context"
	"fmt"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type courseHomeButtonRepo struct {
	db *gorm.DB
}

func NewCourseHomeButtonRepository(db *gorm.DB) repository.CourseHomeButtonRepository {
	return &courseHomeButtonRepo{db: db}
}

func (r *courseHomeButtonRepo) Create(ctx context.Context, button *models.CourseHomeButton) error {
	return r.db.WithContext(ctx).Create(button).Error
}

func (r *courseHomeButtonRepo) FindByID(ctx context.Context, id uint) (*models.CourseHomeButton, error) {
	var button models.CourseHomeButton
	if err := r.db.WithContext(ctx).First(&button, id).Error; err != nil {
		return nil, err
	}
	return &button, nil
}

func (r *courseHomeButtonRepo) Update(ctx context.Context, button *models.CourseHomeButton) error {
	return r.db.WithContext(ctx).Save(button).Error
}

func (r *courseHomeButtonRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.CourseHomeButton{}, id).Error
}

func (r *courseHomeButtonRepo) ListByCourseID(ctx context.Context, courseID uint) ([]models.CourseHomeButton, error) {
	var buttons []models.CourseHomeButton
	if err := r.db.WithContext(ctx).Where("course_id = ?", courseID).Order("position ASC").Find(&buttons).Error; err != nil {
		return nil, err
	}
	return buttons, nil
}

func (r *courseHomeButtonRepo) BulkUpdatePositions(ctx context.Context, courseID uint, positions map[uint]int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for buttonID, position := range positions {
			if err := tx.Model(&models.CourseHomeButton{}).
				Where("id = ? AND course_id = ?", buttonID, courseID).
				Update("position", position).Error; err != nil {
				return fmt.Errorf("failed to update position for button %d: %w", buttonID, err)
			}
		}
		return nil
	})
}
