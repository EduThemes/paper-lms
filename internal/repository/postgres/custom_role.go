package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type customRoleRepo struct {
	db *gorm.DB
}

// NewCustomRoleRepository creates a new CustomRole repository backed by PostgreSQL.
func NewCustomRoleRepository(db *gorm.DB) repository.CustomRoleRepository {
	return &customRoleRepo{db: db}
}

func (r *customRoleRepo) Create(ctx context.Context, role *models.CustomRole) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *customRoleRepo) FindByID(ctx context.Context, id, accountID uint) (*models.CustomRole, error) {
	var role models.CustomRole
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&role, id).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *customRoleRepo) Update(ctx context.Context, role *models.CustomRole) error {
	return r.db.WithContext(ctx).Save(role).Error
}

func (r *customRoleRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.CustomRole{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *customRoleRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CustomRole], error) {
	var roles []models.CustomRole
	var count int64

	query := r.db.WithContext(ctx).Model(&models.CustomRole{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.CustomRole]{
		Items:      roles,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *customRoleRepo) FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.CustomRole, error) {
	var role models.CustomRole
	if err := r.db.WithContext(ctx).Where("account_id = ? AND name = ? AND workflow_state != ?", accountID, name, "deleted").First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *customRoleRepo) ListByBaseRoleType(ctx context.Context, accountID uint, baseRoleType string) ([]models.CustomRole, error) {
	var roles []models.CustomRole
	if err := r.db.WithContext(ctx).Where("account_id = ? AND base_role_type = ? AND workflow_state != ?", accountID, baseRoleType, "deleted").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *customRoleRepo) ListActive(ctx context.Context, accountID uint) ([]models.CustomRole, error) {
	var roles []models.CustomRole
	if err := r.db.WithContext(ctx).Where("account_id = ? AND workflow_state = ?", accountID, "active").Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}
