package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// commentBankItemTenantFilter scopes a comment_bank_items query to a
// tenant via the owning user's account_id. The table carries no direct
// account_id column; user_id → users.account_id is the join. The
// subquery shape matches the parent-table pattern used elsewhere
// (e.g. notifications). accountID==0 disables the filter (no auth-
// internal callers exist for the Delete path today, but the convention
// matches the rest of the codebase).
const commentBankItemTenantFilter = `user_id IN (SELECT id FROM users WHERE account_id = ?)`

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

// Delete — F-012: tenant-scope via the owning user's account_id. A
// cross-tenant id is silently a no-op (zero rows match the WHERE),
// and the calling service surfaces gorm.ErrRecordNotFound when its
// pre-Delete FindByID returns nothing — handler maps to 404 per the
// 13.1.E existence-leak contract. accountID==0 disables the filter
// (no current auth-internal callers for this Delete; convention).
func (r *commentBankItemRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(commentBankItemTenantFilter, accountID)
	}
	return q.Delete(&models.CommentBankItem{}, id).Error
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
