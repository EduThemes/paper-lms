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

// Create persists a new currency definition. Uses a raw parameterized
// INSERT (not gorm.Create) so zero-valued bools — `Monotonic: false` for
// spendable currencies, `VisibleInTopbar: false` for FERPA-protected
// ones — are written explicitly. GORM's `default:` tags otherwise elide
// false in favor of the SQL column DEFAULT TRUE, the same regression
// class as the W2-A seed fix (see seed.go).
func (r *GamificationCurrencyTypeRepo) Create(ctx context.Context, currency *models.GamificationCurrencyType) error {
	const insertSQL = `
		INSERT INTO gamification_currency_types
			(tenant_id, scope_type, scope_id, code, display_label,
			 display_label_plural, icon, color, display_order, spendable,
			 monotonic, ferpa_classification, visible_to_student,
			 visible_in_topbar, system_owned, description,
			 created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now(), now())
		RETURNING id, created_at, updated_at`
	row := r.db.WithContext(ctx).Raw(insertSQL,
		currency.TenantID, currency.ScopeType, currency.ScopeID,
		currency.Code, currency.DisplayLabel, currency.DisplayLabelPlural,
		currency.Icon, currency.Color, currency.DisplayOrder,
		currency.Spendable, currency.Monotonic, currency.FerpaClassification,
		currency.VisibleToStudent, currency.VisibleInTopbar,
		currency.SystemOwned, currency.Description,
	).Row()
	return row.Scan(&currency.ID, &currency.CreatedAt, &currency.UpdatedAt)
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
