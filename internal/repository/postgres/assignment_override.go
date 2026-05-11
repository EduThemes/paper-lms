package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type assignmentOverrideRepo struct {
	db *gorm.DB
}

func NewAssignmentOverrideRepository(db *gorm.DB) repository.AssignmentOverrideRepository {
	return &assignmentOverrideRepo{db: db}
}

func (r *assignmentOverrideRepo) Create(ctx context.Context, override *models.AssignmentOverride) error {
	return r.db.WithContext(ctx).Create(override).Error
}

func (r *assignmentOverrideRepo) FindByID(ctx context.Context, id uint) (*models.AssignmentOverride, error) {
	var override models.AssignmentOverride
	if err := r.db.WithContext(ctx).Where("id = ? AND workflow_state != ?", id, "deleted").First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *assignmentOverrideRepo) Update(ctx context.Context, override *models.AssignmentOverride) error {
	return r.db.WithContext(ctx).Save(override).Error
}

func (r *assignmentOverrideRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.AssignmentOverride{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *assignmentOverrideRepo) ListByAssignmentID(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error) {
	var overrides []models.AssignmentOverride
	if err := r.db.WithContext(ctx).Where("assignment_id = ? AND workflow_state != ?", assignmentID, "deleted").Order("id ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}
