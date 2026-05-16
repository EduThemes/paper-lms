package postgres

import (
	"context"
	"errors"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// FederatedIdentityRepo is the postgres implementation of
// repository.FederatedIdentityRepository.
type FederatedIdentityRepo struct {
	db *gorm.DB
}

func NewFederatedIdentityRepository(db *gorm.DB) *FederatedIdentityRepo {
	return &FederatedIdentityRepo{db: db}
}

func (r *FederatedIdentityRepo) FindByProviderAndSubject(ctx context.Context, providerID uint, externalSubject string) (*models.FederatedIdentity, error) {
	var row models.FederatedIdentity
	err := r.db.WithContext(ctx).
		Where("provider_id = ? AND external_subject = ?", providerID, externalSubject).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func (r *FederatedIdentityRepo) Create(ctx context.Context, fi *models.FederatedIdentity) error {
	return r.db.WithContext(ctx).Create(fi).Error
}

// TouchLastSeen updates last_seen_at and (optionally) replaces the
// claims_snapshot. Passing claimsSnapshot=nil leaves the existing
// snapshot intact — useful for Apple Sign-In which omits claims on
// post-first-consent logins; the original snapshot must survive.
func (r *FederatedIdentityRepo) TouchLastSeen(ctx context.Context, id uint, claimsSnapshot []byte) error {
	updates := map[string]any{"last_seen_at": gorm.Expr("now()")}
	if claimsSnapshot != nil {
		updates["claims_snapshot"] = datatypes.JSON(claimsSnapshot)
	}
	return r.db.WithContext(ctx).
		Model(&models.FederatedIdentity{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *FederatedIdentityRepo) ListForUser(ctx context.Context, userID uint) ([]models.FederatedIdentity, error) {
	var rows []models.FederatedIdentity
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("first_seen_at ASC").
		Find(&rows).Error
	return rows, err
}
