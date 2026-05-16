package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

type OutcomeAlignmentRepository struct {
	db *gorm.DB
}

func NewOutcomeAlignmentRepository(db *gorm.DB) *OutcomeAlignmentRepository {
	return &OutcomeAlignmentRepository{db: db}
}

func (r *OutcomeAlignmentRepository) Create(ctx context.Context, alignment *models.OutcomeAlignment) error {
	return r.db.WithContext(ctx).Create(alignment).Error
}

func (r *OutcomeAlignmentRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.OutcomeAlignment{}, id).Error
}

func (r *OutcomeAlignmentRepository) ListByAssignmentID(ctx context.Context, assignmentID, accountID uint) ([]models.OutcomeAlignment, error) {
	var alignments []models.OutcomeAlignment
	q := r.db.WithContext(ctx).Where("assignment_id = ?", assignmentID)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	err := q.Find(&alignments).Error
	return alignments, err
}

func (r *OutcomeAlignmentRepository) ListByCourseID(ctx context.Context, courseID, accountID uint) ([]models.OutcomeAlignment, error) {
	var alignments []models.OutcomeAlignment
	q := r.db.WithContext(ctx).Where("course_id = ?", courseID)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	err := q.Find(&alignments).Error
	return alignments, err
}
