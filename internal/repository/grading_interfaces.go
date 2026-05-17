package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type GradingStandardRepository interface {
	Create(ctx context.Context, standard *models.GradingStandard) error
	FindByID(ctx context.Context, id uint) (*models.GradingStandard, error)
	Update(ctx context.Context, standard *models.GradingStandard) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint) ([]models.GradingStandard, error)
	FindActiveByCourse(ctx context.Context, courseID uint) (*models.GradingStandard, error)
}

type GradingPeriodGroupRepository interface {
	Create(ctx context.Context, group *models.GradingPeriodGroup) error
	// 13.1.D — tenant scope via direct account_id column. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriodGroup, error)
	Update(ctx context.Context, group *models.GradingPeriodGroup) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.GradingPeriodGroup], error)
}

type GradingPeriodRepository interface {
	Create(ctx context.Context, period *models.GradingPeriod) error
	// 13.1.D — tenant scope via parent grading_period_group's account_id. 0 means no tenant scope (internal callers only).
	FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriod, error)
	Update(ctx context.Context, period *models.GradingPeriod) error
	Delete(ctx context.Context, id uint) error
	ListByGroupID(ctx context.Context, groupID, accountID uint) ([]models.GradingPeriod, error)
}
