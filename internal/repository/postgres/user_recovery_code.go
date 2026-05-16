package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UserRecoveryCodeRepo persists single-use TOTP recovery codes.
// Created in bulk at enrollment; one row marked used per successful
// recovery-code login. The partial index on (user_id) WHERE
// used_at IS NULL keeps the unused-codes lookup cheap.
type UserRecoveryCodeRepo struct {
	db *gorm.DB
}

func NewUserRecoveryCodeRepository(db *gorm.DB) *UserRecoveryCodeRepo {
	return &UserRecoveryCodeRepo{db: db}
}

// CreateBatch writes all generated recovery code hashes in one
// transaction. Either all succeed or none — partial enrollment would
// leave the user with fewer recovery options than they were promised.
func (r *UserRecoveryCodeRepo) CreateBatch(ctx context.Context, userID uint, codeHashes []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear any prior codes — re-enrollment regenerates the set.
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserRecoveryCode{}).Error; err != nil {
			return err
		}
		rows := make([]models.UserRecoveryCode, 0, len(codeHashes))
		for _, h := range codeHashes {
			rows = append(rows, models.UserRecoveryCode{UserID: userID, CodeHash: h})
		}
		return tx.Create(&rows).Error
	})
}

// ListUnusedForUser returns all not-yet-consumed codes for a user.
// The handler iterates them at recovery time, calling VerifyRecoveryCode
// against each until one matches. With 10 codes this is at most 10
// bcrypt comparisons — fast enough for an interactive flow.
func (r *UserRecoveryCodeRepo) ListUnusedForUser(ctx context.Context, userID uint) ([]models.UserRecoveryCode, error) {
	var rows []models.UserRecoveryCode
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL", userID).
		Find(&rows).Error
	return rows, err
}

// MarkUsed atomically transitions a single code from unused → used.
// Returns gorm.ErrRecordNotFound if the row was already used or
// doesn't exist — handler maps that to "invalid recovery code" so
// an attacker can't tell whether they hit a real-but-used code or a
// fabricated one.
func (r *UserRecoveryCodeRepo) MarkUsed(ctx context.Context, id uint) error {
	now := time.Now()
	tx := r.db.WithContext(ctx).
		Model(&models.UserRecoveryCode{}).
		Where("id = ? AND used_at IS NULL", id).
		Updates(map[string]any{"used_at": now})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteAllForUser is called when the user disables MFA — wipes
// their entire recovery-code set so re-enabling generates a fresh
// 10 codes.
func (r *UserRecoveryCodeRepo) DeleteAllForUser(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&models.UserRecoveryCode{}).Error
}
