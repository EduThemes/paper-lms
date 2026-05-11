package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

type quizQuestionOutcomeAlignmentRepo struct {
	db *gorm.DB
}

func NewQuizQuestionOutcomeAlignmentRepository(db *gorm.DB) *quizQuestionOutcomeAlignmentRepo {
	return &quizQuestionOutcomeAlignmentRepo{db: db}
}

func (r *quizQuestionOutcomeAlignmentRepo) Create(ctx context.Context, a *models.QuizQuestionOutcomeAlignment) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *quizQuestionOutcomeAlignmentRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuizQuestionOutcomeAlignment{}, id).Error
}

func (r *quizQuestionOutcomeAlignmentRepo) DeleteByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) error {
	return r.db.WithContext(ctx).
		Where("quiz_question_id = ? AND outcome_id = ?", quizQuestionID, outcomeID).
		Delete(&models.QuizQuestionOutcomeAlignment{}).Error
}

func (r *quizQuestionOutcomeAlignmentRepo) FindByQuestionAndOutcome(ctx context.Context, quizQuestionID, outcomeID uint) (*models.QuizQuestionOutcomeAlignment, error) {
	var a models.QuizQuestionOutcomeAlignment
	if err := r.db.WithContext(ctx).
		Where("quiz_question_id = ? AND outcome_id = ?", quizQuestionID, outcomeID).
		First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *quizQuestionOutcomeAlignmentRepo) ListByQuestionID(ctx context.Context, quizQuestionID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	var items []models.QuizQuestionOutcomeAlignment
	if err := r.db.WithContext(ctx).
		Where("quiz_question_id = ?", quizQuestionID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *quizQuestionOutcomeAlignmentRepo) ListByOutcomeID(ctx context.Context, outcomeID uint) ([]models.QuizQuestionOutcomeAlignment, error) {
	var items []models.QuizQuestionOutcomeAlignment
	if err := r.db.WithContext(ctx).
		Where("outcome_id = ?", outcomeID).
		Order("id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
