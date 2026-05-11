package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type commentBankItemRepo struct {
	db *gorm.DB
}

func NewCommentBankItemRepository(db *gorm.DB) repository.CommentBankItemRepository {
	return &commentBankItemRepo{db: db}
}

func (r *commentBankItemRepo) Create(ctx context.Context, item *models.CommentBankItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *commentBankItemRepo) FindByID(ctx context.Context, id uint) (*models.CommentBankItem, error) {
	var item models.CommentBankItem
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *commentBankItemRepo) Update(ctx context.Context, item *models.CommentBankItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *commentBankItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.CommentBankItem{}, id).Error
}

func (r *commentBankItemRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CommentBankItem], error) {
	var items []models.CommentBankItem
	var count int64

	query := r.db.WithContext(ctx).Model(&models.CommentBankItem{}).Where("user_id = ?", userID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.CommentBankItem]{
		Items:      items,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *commentBankItemRepo) SearchByUser(ctx context.Context, userID uint, query string) ([]models.CommentBankItem, error) {
	var items []models.CommentBankItem
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND comment ILIKE ?", userID, "%"+query+"%").
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
