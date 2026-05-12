package postgres

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"gorm.io/gorm"
)

// GamificationCurrencyTypeRepo persists user-defined currency definitions.
type GamificationCurrencyTypeRepo struct {
	db *gorm.DB
}

func NewGamificationCurrencyTypeRepository(db *gorm.DB) *GamificationCurrencyTypeRepo {
	return &GamificationCurrencyTypeRepo{db: db}
}

func (r *GamificationCurrencyTypeRepo) Create(ctx context.Context, currency *models.GamificationCurrencyType) error {
	return r.db.WithContext(ctx).Create(currency).Error
}

func (r *GamificationCurrencyTypeRepo) FindByID(ctx context.Context, id uint) (*models.GamificationCurrencyType, error) {
	var currency models.GamificationCurrencyType
	if err := r.db.WithContext(ctx).First(&currency, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &currency, nil
}

func (r *GamificationCurrencyTypeRepo) FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error) {
	var currency models.GamificationCurrencyType
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND scope_type = ? AND scope_id = ? AND code = ?", tenantID, scopeType, scopeID, code).
		First(&currency).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &currency, nil
}

func (r *GamificationCurrencyTypeRepo) Update(ctx context.Context, currency *models.GamificationCurrencyType) error {
	return r.db.WithContext(ctx).Save(currency).Error
}

// Delete refuses to remove a system-owned row (xp, gems, mastery_points,
// reputation). Those rows can be renamed but not destroyed.
func (r *GamificationCurrencyTypeRepo) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND system_owned = ?", id, false).
		Delete(&models.GamificationCurrencyType{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("currency type not found or is system-owned")
	}
	return nil
}

func (r *GamificationCurrencyTypeRepo) ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	var currencies []models.GamificationCurrencyType
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("display_order ASC, id ASC").
		Find(&currencies).Error
	return currencies, err
}

func (r *GamificationCurrencyTypeRepo) ListInTopbar(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	var currencies []models.GamificationCurrencyType
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND visible_in_topbar = ?", tenantID, true).
		Order("display_order ASC, id ASC").
		Find(&currencies).Error
	return currencies, err
}
