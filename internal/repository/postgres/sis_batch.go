package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type sisBatchRepo struct {
	db *gorm.DB
}

func NewSISBatchRepository(db *gorm.DB) repository.SISBatchRepository {
	return &sisBatchRepo{db: db}
}

func (r *sisBatchRepo) Create(ctx context.Context, batch *models.SISBatch) error {
	return r.db.WithContext(ctx).Create(batch).Error
}

func (r *sisBatchRepo) FindByID(ctx context.Context, id uint) (*models.SISBatch, error) {
	var batch models.SISBatch
	if err := r.db.WithContext(ctx).First(&batch, id).Error; err != nil {
		return nil, err
	}
	return &batch, nil
}

func (r *sisBatchRepo) Update(ctx context.Context, batch *models.SISBatch) error {
	return r.db.WithContext(ctx).Save(batch).Error
}

func (r *sisBatchRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SISBatch], error) {
	var batches []models.SISBatch
	var count int64

	query := r.db.WithContext(ctx).Model(&models.SISBatch{}).Where("account_id = ?", accountID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&batches).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.SISBatch]{
		Items:      batches,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
