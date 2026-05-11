package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// PairingCodeRepository is the consumer-side interface used by the service
// layer. The implementation lives in this package; if a shared interface is
// desired it can be added to repository/interfaces.go by the main thread.
type PairingCodeRepository interface {
	Create(ctx context.Context, code *models.PairingCode) error
	FindByCode(ctx context.Context, code string) (*models.PairingCode, error)
	FindByID(ctx context.Context, id uint) (*models.PairingCode, error)
	MarkRedeemed(ctx context.Context, id uint, redeemedAt time.Time) error
	ListActiveByUserID(ctx context.Context, userID uint, now time.Time) ([]models.PairingCode, error)
	Delete(ctx context.Context, id uint) error

	// WithTx returns a repository bound to the given transaction.
	WithTx(tx *gorm.DB) PairingCodeRepository
	// DB exposes the underlying *gorm.DB for service-layer transactions.
	DB() *gorm.DB
}

type pairingCodeRepo struct {
	db *gorm.DB
}

func NewPairingCodeRepository(db *gorm.DB) PairingCodeRepository {
	return &pairingCodeRepo{db: db}
}

func (r *pairingCodeRepo) DB() *gorm.DB { return r.db }

func (r *pairingCodeRepo) WithTx(tx *gorm.DB) PairingCodeRepository {
	return &pairingCodeRepo{db: tx}
}

func (r *pairingCodeRepo) Create(ctx context.Context, code *models.PairingCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

func (r *pairingCodeRepo) FindByCode(ctx context.Context, code string) (*models.PairingCode, error) {
	var pc models.PairingCode
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&pc).Error; err != nil {
		return nil, err
	}
	return &pc, nil
}

func (r *pairingCodeRepo) FindByID(ctx context.Context, id uint) (*models.PairingCode, error) {
	var pc models.PairingCode
	if err := r.db.WithContext(ctx).First(&pc, id).Error; err != nil {
		return nil, err
	}
	return &pc, nil
}

func (r *pairingCodeRepo) MarkRedeemed(ctx context.Context, id uint, redeemedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.PairingCode{}).
		Where("id = ? AND redeemed_at IS NULL", id).
		Update("redeemed_at", redeemedAt).Error
}

func (r *pairingCodeRepo) ListActiveByUserID(ctx context.Context, userID uint, now time.Time) ([]models.PairingCode, error) {
	var codes []models.PairingCode
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND redeemed_at IS NULL AND expires_at > ?", userID, now).
		Order("created_at DESC").
		Find(&codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

func (r *pairingCodeRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.PairingCode{}, id).Error
}
