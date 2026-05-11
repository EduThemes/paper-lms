package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Analytics

type PageViewRepository interface {
	Create(ctx context.Context, pageView *models.PageView) error
	FindByID(ctx context.Context, id uint) (*models.PageView, error)
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.PageView], error)
	CountByContextGrouped(ctx context.Context, contextType string, contextID uint) ([]map[string]interface{}, error)
	SumInteractionByUserAndContext(ctx context.Context, userID uint, contextType string, contextID uint) (int64, error)
}
