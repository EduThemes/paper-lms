package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type quizRepo struct {
	db *gorm.DB
}

func NewQuizRepository(db *gorm.DB) *quizRepo {
	return &quizRepo{db: db}
}

func (r *quizRepo) Create(ctx context.Context, quiz *models.Quiz) error {
	return r.db.WithContext(ctx).Create(quiz).Error
}

func (r *quizRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Quiz, error) {
	var quiz models.Quiz
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("course_id IN (SELECT id FROM courses WHERE account_id = ?)", accountID)
	}
	if err := q.First(&quiz, id).Error; err != nil {
		return nil, err
	}
	return &quiz, nil
}

func (r *quizRepo) Update(ctx context.Context, quiz *models.Quiz) error {
	return r.db.WithContext(ctx).Save(quiz).Error
}

func (r *quizRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Quiz{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *quizRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Quiz], error) {
	var items []models.Quiz
	var totalCount int64

	query := r.db.WithContext(ctx).Model(&models.Quiz{}).Where("course_id = ? AND workflow_state != 'deleted'", courseID)
	query.Count(&totalCount)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Quiz]{
		Items:      items,
		TotalCount: totalCount,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
