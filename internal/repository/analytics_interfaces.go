package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Analytics

type PageViewRepository interface {
	Create(ctx context.Context, pageView *models.PageView) error
	// 13.1.D — tenant scope via polymorphic context_type/context_id.
	// Page views can attach to any contextable entity in any tenant.
	FindByID(ctx context.Context, id, accountID uint) (*models.PageView, error)
	ListByUserID(ctx context.Context, userID, accountID uint, params PaginationParams) (*PaginatedResult[models.PageView], error)
	CountByContextGrouped(ctx context.Context, contextType string, contextID uint) ([]map[string]interface{}, error)
	SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error)
}
