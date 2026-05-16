package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type roleOverrideRepo struct {
	db *gorm.DB
}

// NewRoleOverrideRepository creates a new RoleOverride repository backed by PostgreSQL.
func NewRoleOverrideRepository(db *gorm.DB) repository.RoleOverrideRepository {
	return &roleOverrideRepo{db: db}
}

func (r *roleOverrideRepo) Create(ctx context.Context, override *models.RoleOverride) error {
	return r.db.WithContext(ctx).Create(override).Error
}

func (r *roleOverrideRepo) FindByID(ctx context.Context, id, accountID uint) (*models.RoleOverride, error) {
	var override models.RoleOverride
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&override, id).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *roleOverrideRepo) Update(ctx context.Context, override *models.RoleOverride) error {
	return r.db.WithContext(ctx).Save(override).Error
}

func (r *roleOverrideRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.RoleOverride{}, id).Error
}

func (r *roleOverrideRepo) ListByRoleID(ctx context.Context, roleID uint) ([]models.RoleOverride, error) {
	var overrides []models.RoleOverride
	if err := r.db.WithContext(ctx).Where("role_id = ?", roleID).Order("permission ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

func (r *roleOverrideRepo) FindByRoleAndPermission(ctx context.Context, roleID uint, permission string) (*models.RoleOverride, error) {
	var override models.RoleOverride
	if err := r.db.WithContext(ctx).Where("role_id = ? AND permission = ?", roleID, permission).First(&override).Error; err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *roleOverrideRepo) ListByAccountID(ctx context.Context, accountID uint) ([]models.RoleOverride, error) {
	var overrides []models.RoleOverride
	if err := r.db.WithContext(ctx).Where("account_id = ?", accountID).Order("role_id ASC, permission ASC").Find(&overrides).Error; err != nil {
		return nil, err
	}
	return overrides, nil
}

func (r *roleOverrideRepo) BulkUpsert(ctx context.Context, overrides []models.RoleOverride) error {
	if len(overrides) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "role_id"}, {Name: "permission"}, {Name: "context_type"}, {Name: "context_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "locked", "updated_at"}),
	}).Create(&overrides).Error
}
