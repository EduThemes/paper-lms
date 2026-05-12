package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

type quizItemBankItemRepo struct {
	db *gorm.DB
}

func NewQuizItemBankItemRepository(db *gorm.DB) *quizItemBankItemRepo {
	return &quizItemBankItemRepo{db: db}
}

func (r *quizItemBankItemRepo) Create(ctx context.Context, item *models.QuizItemBankItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *quizItemBankItemRepo) FindByID(ctx context.Context, id uint) (*models.QuizItemBankItem, error) {
	var item models.QuizItemBankItem
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *quizItemBankItemRepo) Update(ctx context.Context, item *models.QuizItemBankItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *quizItemBankItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.QuizItemBankItem{}, id).Error
}

func (r *quizItemBankItemRepo) ListByBankID(ctx context.Context, bankID uint) ([]models.QuizItemBankItem, error) {
	var items []models.QuizItemBankItem
	if err := r.db.WithContext(ctx).
		Where("bank_id = ?", bankID).
		Order("position ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
