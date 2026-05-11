package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type quizSubmissionRepo struct {
	db *gorm.DB
}

func NewQuizSubmissionRepository(db *gorm.DB) *quizSubmissionRepo {
	return &quizSubmissionRepo{db: db}
}

func (r *quizSubmissionRepo) Create(ctx context.Context, submission *models.QuizSubmission) error {
	return r.db.WithContext(ctx).Create(submission).Error
}

func (r *quizSubmissionRepo) FindByID(ctx context.Context, id uint) (*models.QuizSubmission, error) {
	var submission models.QuizSubmission
	if err := r.db.WithContext(ctx).First(&submission, id).Error; err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *quizSubmissionRepo) Update(ctx context.Context, submission *models.QuizSubmission) error {
	return r.db.WithContext(ctx).Save(submission).Error
}

func (r *quizSubmissionRepo) FindByQuizAndUser(ctx context.Context, quizID, userID uint) (*models.QuizSubmission, error) {
	var submission models.QuizSubmission
	if err := r.db.WithContext(ctx).Where("quiz_id = ? AND user_id = ?", quizID, userID).Order("attempt DESC").First(&submission).Error; err != nil {
		return nil, err
	}
	return &submission, nil
}

func (r *quizSubmissionRepo) ListByQuizID(ctx context.Context, quizID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizSubmission], error) {
	var submissions []models.QuizSubmission
	var count int64

	query := r.db.WithContext(ctx).Model(&models.QuizSubmission{}).Where("quiz_id = ?", quizID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&submissions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.QuizSubmission]{
		Items:      submissions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *quizSubmissionRepo) ListCompletedByQuizID(ctx context.Context, quizID uint) ([]models.QuizSubmission, error) {
	var submissions []models.QuizSubmission
	if err := r.db.WithContext(ctx).
		Where("quiz_id = ? AND workflow_state IN (?)", quizID, []string{"complete", "pending_review"}).
		Order("id ASC").
		Find(&submissions).Error; err != nil {
		return nil, err
	}
	return submissions, nil
}
