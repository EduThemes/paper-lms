package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UserWebauthnCredentialRepo persists registered passkey credentials.
// See migration 000049 + models.UserWebauthnCredential.
type UserWebauthnCredentialRepo struct {
	db *gorm.DB
}

func NewUserWebauthnCredentialRepository(db *gorm.DB) *UserWebauthnCredentialRepo {
	return &UserWebauthnCredentialRepo{db: db}
}

func (r *UserWebauthnCredentialRepo) Create(ctx context.Context, cred *models.UserWebauthnCredential) error {
	return r.db.WithContext(ctx).Create(cred).Error
}

func (r *UserWebauthnCredentialRepo) FindByCredentialID(ctx context.Context, credentialID []byte) (*models.UserWebauthnCredential, error) {
	var row models.UserWebauthnCredential
	err := r.db.WithContext(ctx).Where("credential_id = ?", credentialID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *UserWebauthnCredentialRepo) FindByID(ctx context.Context, id uint) (*models.UserWebauthnCredential, error) {
	var row models.UserWebauthnCredential
	err := r.db.WithContext(ctx).First(&row, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *UserWebauthnCredentialRepo) ListForUser(ctx context.Context, userID uint) ([]models.UserWebauthnCredential, error) {
	var rows []models.UserWebauthnCredential
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *UserWebauthnCredentialRepo) UpdateSignCount(ctx context.Context, id uint, newSignCount uint32) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.UserWebauthnCredential{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"sign_count":   newSignCount,
			"last_used_at": now,
		}).Error
}

// UpdateNickname scopes the update to (id, user_id) so a stolen id
// from one user can't rename another user's passkey via a forged
// request.
func (r *UserWebauthnCredentialRepo) UpdateNickname(ctx context.Context, id, userID uint, nickname string) error {
	tx := r.db.WithContext(ctx).
		Model(&models.UserWebauthnCredential{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("nickname", nickname)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *UserWebauthnCredentialRepo) Delete(ctx context.Context, id, userID uint) error {
	tx := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.UserWebauthnCredential{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
