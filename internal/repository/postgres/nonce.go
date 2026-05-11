package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type nonceRepo struct {
	db *gorm.DB
}

func NewNonceRepository(db *gorm.DB) repository.NonceRepository {
	return &nonceRepo{db: db}
}

func (r *nonceRepo) Create(ctx context.Context, nonce *models.Nonce) error {
	return r.db.WithContext(ctx).Create(nonce).Error
}

func (r *nonceRepo) Exists(ctx context.Context, value string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Nonce{}).Where("value = ? AND expires_at > ?", value, time.Now()).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *nonceRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.Nonce{}).Error
}
