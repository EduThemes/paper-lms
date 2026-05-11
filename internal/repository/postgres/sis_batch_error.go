package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type sisBatchErrorRepo struct {
	db *gorm.DB
}

func NewSISBatchErrorRepository(db *gorm.DB) repository.SISBatchErrorRepository {
	return &sisBatchErrorRepo{db: db}
}

func (r *sisBatchErrorRepo) Create(ctx context.Context, batchError *models.SISBatchError) error {
	return r.db.WithContext(ctx).Create(batchError).Error
}

func (r *sisBatchErrorRepo) ListByBatchID(ctx context.Context, batchID uint) ([]models.SISBatchError, error) {
	var errors []models.SISBatchError
	if err := r.db.WithContext(ctx).Where("sis_batch_id = ?", batchID).Order("row ASC").Find(&errors).Error; err != nil {
		return nil, err
	}
	return errors, nil
}
