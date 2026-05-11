package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Custom Roles & Granular Permissions

type CustomRoleRepository interface {
	Create(ctx context.Context, role *models.CustomRole) error
	FindByID(ctx context.Context, id uint) (*models.CustomRole, error)
	Update(ctx context.Context, role *models.CustomRole) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.CustomRole], error)
	FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.CustomRole, error)
	ListByBaseRoleType(ctx context.Context, accountID uint, baseRoleType string) ([]models.CustomRole, error)
	ListActive(ctx context.Context, accountID uint) ([]models.CustomRole, error)
}

type RoleOverrideRepository interface {
	Create(ctx context.Context, override *models.RoleOverride) error
	FindByID(ctx context.Context, id uint) (*models.RoleOverride, error)
	Update(ctx context.Context, override *models.RoleOverride) error
	Delete(ctx context.Context, id uint) error
	ListByRoleID(ctx context.Context, roleID uint) ([]models.RoleOverride, error)
	FindByRoleAndPermission(ctx context.Context, roleID uint, permission string) (*models.RoleOverride, error)
	ListByAccountID(ctx context.Context, accountID uint) ([]models.RoleOverride, error)
	BulkUpsert(ctx context.Context, overrides []models.RoleOverride) error
}
