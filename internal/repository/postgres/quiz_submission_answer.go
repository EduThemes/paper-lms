package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

type quizSubmissionAnswerRepo struct {
	db *gorm.DB
}

func NewQuizSubmissionAnswerRepository(db *gorm.DB) *quizSubmissionAnswerRepo {
	return &quizSubmissionAnswerRepo{db: db}
}

func (r *quizSubmissionAnswerRepo) Create(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	return r.db.WithContext(ctx).Create(answer).Error
}

func (r *quizSubmissionAnswerRepo) BulkCreate(ctx context.Context, answers []models.QuizSubmissionAnswer) error {
	if len(answers) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&answers).Error
}

func (r *quizSubmissionAnswerRepo) FindByID(ctx context.Context, id uint) (*models.QuizSubmissionAnswer, error) {
	var answer models.QuizSubmissionAnswer
	if err := r.db.WithContext(ctx).First(&answer, id).Error; err != nil {
		return nil, err
	}
	return &answer, nil
}

func (r *quizSubmissionAnswerRepo) Update(ctx context.Context, answer *models.QuizSubmissionAnswer) error {
	return r.db.WithContext(ctx).Save(answer).Error
}

func (r *quizSubmissionAnswerRepo) ListBySubmissionID(ctx context.Context, submissionID uint) ([]models.QuizSubmissionAnswer, error) {
	var answers []models.QuizSubmissionAnswer
	if err := r.db.WithContext(ctx).Where("quiz_submission_id = ?", submissionID).Order("id ASC").Find(&answers).Error; err != nil {
		return nil, err
	}
	return answers, nil
}

func (r *quizSubmissionAnswerRepo) FindBySubmissionAndQuestion(ctx context.Context, submissionID, questionID uint) (*models.QuizSubmissionAnswer, error) {
	var answer models.QuizSubmissionAnswer
	if err := r.db.WithContext(ctx).Where("quiz_submission_id = ? AND question_id = ?", submissionID, questionID).First(&answer).Error; err != nil {
		return nil, err
	}
	return &answer, nil
}

func (r *quizSubmissionAnswerRepo) ListBySubmissionIDs(ctx context.Context, submissionIDs []uint) ([]models.QuizSubmissionAnswer, error) {
	if len(submissionIDs) == 0 {
		return nil, nil
	}
	var answers []models.QuizSubmissionAnswer
	if err := r.db.WithContext(ctx).Where("quiz_submission_id IN (?)", submissionIDs).Order("id ASC").Find(&answers).Error; err != nil {
		return nil, err
	}
	return answers, nil
}
