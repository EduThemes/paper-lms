package postgres

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type settingRepo struct {
	db *gorm.DB
}

// NewSettingRepository returns the Postgres-backed settings store.
func NewSettingRepository(db *gorm.DB) repository.SettingRepository {
	return &settingRepo{db: db}
}

func (r *settingRepo) FindByScope(ctx context.Context, scopeType string, scopeID uint, key string) (*models.Setting, error) {
	var s models.Setting
	err := r.db.WithContext(ctx).
		Where("scope_type = ? AND scope_id = ? AND key = ?", scopeType, scopeID, key).
		First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, repository.ErrSettingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *settingRepo) ListByScope(ctx context.Context, scopeType string, scopeID uint) ([]models.Setting, error) {
	var out []models.Setting
	if err := r.db.WithContext(ctx).
		Where("scope_type = ? AND scope_id = ?", scopeType, scopeID).
		Order("key ASC").
		Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Upsert writes the row at (scope_type, scope_id, key), overwriting
// any existing entry. The DB constraint settings_scope_unique is the
// natural-key target. UpdatedAt is stamped here rather than relying on
// GORM's autoUpdateTime so the conflict-update branch carries it too.
func (r *settingRepo) Upsert(ctx context.Context, setting *models.Setting) error {
	now := time.Now()
	setting.UpdatedAt = now
	if setting.CreatedAt.IsZero() {
		setting.CreatedAt = now
	}
	if setting.ValueType == "" {
		setting.ValueType = "string"
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "scope_type"},
			{Name: "scope_id"},
			{Name: "key"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"value_plain",
			"value_encrypted",
			"value_type",
			"updated_by",
			"updated_at",
		}),
	}).Create(setting).Error
}

func (r *settingRepo) Delete(ctx context.Context, scopeType string, scopeID uint, key string) error {
	return r.db.WithContext(ctx).
		Where("scope_type = ? AND scope_id = ? AND key = ?", scopeType, scopeID, key).
		Delete(&models.Setting{}).Error
}
