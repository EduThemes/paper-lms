package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// FeatureFlagRepository is the persistence boundary for feature flag overrides.
// We deliberately don't depend on the central interfaces.go here — the
// PATCH.md instructs the integrator to add an interface there mirroring this
// shape. The methods below match it exactly.
type FeatureFlagRepository interface {
	FindByContext(ctx context.Context, contextType string, contextID uint, feature string) (*models.FeatureFlag, error)
	ListByContext(ctx context.Context, contextType string, contextID uint) ([]models.FeatureFlag, error)
	Upsert(ctx context.Context, flag *models.FeatureFlag) error
	Delete(ctx context.Context, id uint) error
	DeleteByContext(ctx context.Context, contextType string, contextID uint, feature string) error
}

type featureFlagRepo struct {
	db *gorm.DB
}

// NewFeatureFlagRepository constructs the GORM-backed feature flag repo.
func NewFeatureFlagRepository(db *gorm.DB) FeatureFlagRepository {
	return &featureFlagRepo{db: db}
}

func (r *featureFlagRepo) FindByContext(ctx context.Context, contextType string, contextID uint, feature string) (*models.FeatureFlag, error) {
	var flag models.FeatureFlag
	err := r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ? AND feature = ?", contextType, contextID, feature).
		First(&flag).Error
	if err != nil {
		return nil, err
	}
	return &flag, nil
}

func (r *featureFlagRepo) ListByContext(ctx context.Context, contextType string, contextID uint) ([]models.FeatureFlag, error) {
	var flags []models.FeatureFlag
	if err := r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ?", contextType, contextID).
		Order("feature ASC").
		Find(&flags).Error; err != nil {
		return nil, err
	}
	return flags, nil
}

// Upsert writes the flag — creating it if absent, updating state if present.
// We use a (context_type, context_id, feature) tuple as the natural key.
func (r *featureFlagRepo) Upsert(ctx context.Context, flag *models.FeatureFlag) error {
	existing, err := r.FindByContext(ctx, flag.ContextType, flag.ContextID, flag.Feature)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if existing != nil {
		existing.State = flag.State
		if err := r.db.WithContext(ctx).Save(existing).Error; err != nil {
			return err
		}
		*flag = *existing
		return nil
	}
	return r.db.WithContext(ctx).Create(flag).Error
}

func (r *featureFlagRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.FeatureFlag{}, id).Error
}

func (r *featureFlagRepo) DeleteByContext(ctx context.Context, contextType string, contextID uint, feature string) error {
	return r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ? AND feature = ?", contextType, contextID, feature).
		Delete(&models.FeatureFlag{}).Error
}
