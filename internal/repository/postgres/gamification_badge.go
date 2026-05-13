package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

// GamificationBadgeRepo persists badge definitions. See model + migration
// 000041 docs for shape and contracts.
type GamificationBadgeRepo struct {
	db *gorm.DB
}

func NewGamificationBadgeRepository(db *gorm.DB) *GamificationBadgeRepo {
	return &GamificationBadgeRepo{db: db}
}

// Create persists a new badge definition. Uses a raw parameterized
// INSERT (not gorm.Create) because GORM's `default:` tag handling would
// elide `InternalOnly: false` and `SystemOwned: true` zero values against
// the migration defaults — same regression class as the W2-A seed and
// W2-B currency Create fixes. ON CONFLICT DO NOTHING RETURNING collapses
// duplicate detection into a single atomic statement; sql.ErrNoRows is
// translated to repository.ErrBadgeDuplicate so the handler maps cleanly
// to a 409.
func (r *GamificationBadgeRepo) Create(ctx context.Context, badge *models.GamificationBadge) error {
	const insertSQL = `
		INSERT INTO gamification_badges
			(tenant_id, scope_type, scope_id, code, name, description, icon,
			 image_url, color, internal_only, system_owned, audience_level,
			 created_by, created_at, updated_at)
		VALUES
			(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now(), now())
		ON CONFLICT ON CONSTRAINT uniq_gam_badge_scope_code DO NOTHING
		RETURNING id, created_at, updated_at`
	row := r.db.WithContext(ctx).Raw(insertSQL,
		badge.TenantID, badge.ScopeType, badge.ScopeID, badge.Code,
		badge.Name, badge.Description, badge.Icon, badge.ImageURL,
		badge.Color, badge.InternalOnly, badge.SystemOwned,
		badge.AudienceLevel, badge.CreatedBy,
	).Row()
	if err := row.Scan(&badge.ID, &badge.CreatedAt, &badge.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ErrBadgeDuplicate
		}
		return err
	}
	return nil
}

func (r *GamificationBadgeRepo) FindByID(ctx context.Context, id uint) (*models.GamificationBadge, error) {
	var badge models.GamificationBadge
	if err := r.db.WithContext(ctx).First(&badge, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &badge, nil
}

func (r *GamificationBadgeRepo) FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationBadge, error) {
	var badge models.GamificationBadge
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND scope_type = ? AND scope_id = ? AND code = ?", tenantID, scopeType, scopeID, code).
		First(&badge).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &badge, nil
}

// Update uses db.Save (not db.Updates) so every column — including
// zero-valued bools like InternalOnly: false — is written explicitly.
// Mirrors the W2-B fix on GamificationCurrencyTypeRepo.Update.
func (r *GamificationBadgeRepo) Update(ctx context.Context, badge *models.GamificationBadge) error {
	return r.db.WithContext(ctx).Save(badge).Error
}

// Delete refuses to remove a system_owned row. ON DELETE CASCADE on
// gamification_badge_awards.badge_id means deleting a badge also wipes
// every award of it — admin-flow callers should warn before calling.
func (r *GamificationBadgeRepo) Delete(ctx context.Context, id uint) error {
	res := r.db.WithContext(ctx).
		Where("id = ? AND system_owned = ?", id, false).
		Delete(&models.GamificationBadge{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("badge not found or is system-owned")
	}
	return nil
}

func (r *GamificationBadgeRepo) ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationBadge, error) {
	var badges []models.GamificationBadge
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("name ASC, id ASC").
		Find(&badges).Error
	return badges, err
}
