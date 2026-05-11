package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type assignmentOverrideStudentRepo struct {
	db *gorm.DB
}

func NewAssignmentOverrideStudentRepository(db *gorm.DB) repository.AssignmentOverrideStudentRepository {
	return &assignmentOverrideStudentRepo{db: db}
}

func (r *assignmentOverrideStudentRepo) Create(ctx context.Context, student *models.AssignmentOverrideStudent) error {
	return r.db.WithContext(ctx).Create(student).Error
}

func (r *assignmentOverrideStudentRepo) Delete(ctx context.Context, overrideID, userID uint) error {
	return r.db.WithContext(ctx).Where("assignment_override_id = ? AND user_id = ?", overrideID, userID).Delete(&models.AssignmentOverrideStudent{}).Error
}

func (r *assignmentOverrideStudentRepo) ListByOverrideID(ctx context.Context, overrideID uint) ([]models.AssignmentOverrideStudent, error) {
	var students []models.AssignmentOverrideStudent
	if err := r.db.WithContext(ctx).Where("assignment_override_id = ?", overrideID).Order("id ASC").Find(&students).Error; err != nil {
		return nil, err
	}
	return students, nil
}

func (r *assignmentOverrideStudentRepo) ListByUserAndAssignment(ctx context.Context, userID, assignmentID uint) ([]models.AssignmentOverrideStudent, error) {
	var students []models.AssignmentOverrideStudent
	if err := r.db.WithContext(ctx).Where("user_id = ? AND assignment_id = ?", userID, assignmentID).Find(&students).Error; err != nil {
		return nil, err
	}
	return students, nil
}
