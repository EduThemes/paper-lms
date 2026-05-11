package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type quizItemBankRepo struct {
	db *gorm.DB
}

func NewQuizItemBankRepository(db *gorm.DB) *quizItemBankRepo {
	return &quizItemBankRepo{db: db}
}

func (r *quizItemBankRepo) Create(ctx context.Context, bank *models.QuizItemBank) error {
	return r.db.WithContext(ctx).Create(bank).Error
}

func (r *quizItemBankRepo) FindByID(ctx context.Context, id uint) (*models.QuizItemBank, error) {
	var bank models.QuizItemBank
	if err := r.db.WithContext(ctx).First(&bank, id).Error; err != nil {
		return nil, err
	}
	return &bank, nil
}

func (r *quizItemBankRepo) Update(ctx context.Context, bank *models.QuizItemBank) error {
	return r.db.WithContext(ctx).Save(bank).Error
}

func (r *quizItemBankRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuizItemBank{}, id).Error
}

func (r *quizItemBankRepo) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.QuizItemBank], error) {
	var items []models.QuizItemBank
	var total int64

	query := r.db.WithContext(ctx).Model(&models.QuizItemBank{}).Where("course_id = ?", courseID)
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (params.Page - 1) * params.PerPage
	if err := query.Order("created_at DESC").Offset(offset).Limit(params.PerPage).Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.QuizItemBank]{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
