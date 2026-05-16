package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// SharedContentFilters narrows a Browse query.
type SharedContentFilters struct {
	ResourceType string
	Subject      string
	GradeLevel   string
	Search       string
	AuthorUserID uint
}

// SharedContentRepository is the persistence boundary for the Commons
// content library. Defined here (NOT in interfaces.go) so this feature
// can be wired without touching the shared interface file.
type SharedContentRepository interface {
	Create(ctx context.Context, item *models.SharedContent) error
	// 13.1.D — direct account_id column.
	FindByID(ctx context.Context, id, accountID uint) (*models.SharedContent, error)
	Update(ctx context.Context, item *models.SharedContent) error
	Delete(ctx context.Context, id uint) error
	ListByAccount(ctx context.Context, accountID uint, filters SharedContentFilters, params PaginationParams) (*PaginatedResult[models.SharedContent], error)
	IncrementDownloadCount(ctx context.Context, id uint) error
	ToggleFavorite(ctx context.Context, sharedContentID, userID uint) (favorited bool, err error)
	IsFavorited(ctx context.Context, sharedContentID, userID uint) (bool, error)
	ListUserFavorites(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.SharedContent], error)
}
