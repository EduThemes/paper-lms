package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type moduleItemRepo struct {
	db *gorm.DB
}

func NewModuleItemRepository(db *gorm.DB) repository.ModuleItemRepository {
	return &moduleItemRepo{db: db}
}

func (r *moduleItemRepo) Create(ctx context.Context, item *models.ContentTag) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *moduleItemRepo) FindByID(ctx context.Context, id uint) (*models.ContentTag, error) {
	var item models.ContentTag
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *moduleItemRepo) Update(ctx context.Context, item *models.ContentTag) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *moduleItemRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.ContentTag{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *moduleItemRepo) ListByModuleID(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	return r.listByModule(ctx, r.db.WithContext(ctx).Model(&models.ContentTag{}).Where("context_module_id = ? AND workflow_state != ?", moduleID, "deleted"), params)
}

// ListPublishedByModuleID returns only published (active) items — the
// student-facing view (B045).
func (r *moduleItemRepo) ListPublishedByModuleID(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	return r.listByModule(ctx, r.db.WithContext(ctx).Model(&models.ContentTag{}).Where("context_module_id = ? AND workflow_state = ?", moduleID, models.ContentTagActive), params)
}

func (r *moduleItemRepo) listByModule(ctx context.Context, query *gorm.DB, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	var items []models.ContentTag
	var count int64

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC").Find(&items).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ContentTag]{
		Items:      items,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *moduleItemRepo) ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range itemIDs {
			if err := tx.Model(&models.ContentTag{}).
				Where("id = ? AND context_module_id = ?", id, moduleID).
				Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *moduleItemRepo) MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error {
	return r.db.WithContext(ctx).Model(&models.ContentTag{}).
		Where("id = ?", itemID).
		Updates(map[string]interface{}{
			"context_module_id": targetModuleID,
			"position":         position,
		}).Error
}
