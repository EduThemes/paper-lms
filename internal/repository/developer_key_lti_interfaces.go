package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type DeveloperKeyRepository interface {
	Create(ctx context.Context, key *models.DeveloperKey) error
	FindByID(ctx context.Context, id uint) (*models.DeveloperKey, error)
	FindByClientID(ctx context.Context, clientID string) (*models.DeveloperKey, error)
	Update(ctx context.Context, key *models.DeveloperKey) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.DeveloperKey], error)
}

type AccessTokenRepository interface {
	Create(ctx context.Context, token *models.AccessToken) error
	FindByID(ctx context.Context, id uint) (*models.AccessToken, error)
	FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error)
	FindByRefreshToken(ctx context.Context, refreshToken string) (*models.AccessToken, error)
	Update(ctx context.Context, token *models.AccessToken) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.AccessToken], error)
	DeleteExpired(ctx context.Context) error
}

type NonceRepository interface {
	Create(ctx context.Context, nonce *models.Nonce) error
	Exists(ctx context.Context, value string) (bool, error)
	DeleteExpired(ctx context.Context) error
}

type LTIToolConfigurationRepository interface {
	Create(ctx context.Context, config *models.LTIToolConfiguration) error
	FindByID(ctx context.Context, id uint) (*models.LTIToolConfiguration, error)
	FindByDeveloperKeyID(ctx context.Context, devKeyID uint) (*models.LTIToolConfiguration, error)
	Update(ctx context.Context, config *models.LTIToolConfiguration) error
	Delete(ctx context.Context, id uint) error
}

type ContextExternalToolRepository interface {
	Create(ctx context.Context, tool *models.ContextExternalTool) error
	// FindByID — 13.1.D: context-polymorphic tenant scope.
	// context_type='Course' → JOIN courses; context_type='Account' → direct.
	FindByID(ctx context.Context, id, accountID uint) (*models.ContextExternalTool, error)
	Update(ctx context.Context, tool *models.ContextExternalTool) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.ContextExternalTool], error)
}

type LTIResourceLinkRepository interface {
	Create(ctx context.Context, link *models.LTIResourceLink) error
	FindByID(ctx context.Context, id uint) (*models.LTIResourceLink, error)
	FindByResourceLinkID(ctx context.Context, resourceLinkID string) (*models.LTIResourceLink, error)
	Delete(ctx context.Context, id uint) error
}

type LTILineItemRepository interface {
	Create(ctx context.Context, item *models.LTILineItem) error
	FindByID(ctx context.Context, id uint) (*models.LTILineItem, error)
	Update(ctx context.Context, item *models.LTILineItem) error
	Delete(ctx context.Context, id uint) error
	ListByCourse(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.LTILineItem], error)
	FindByAssignmentID(ctx context.Context, assignmentID uint) (*models.LTILineItem, error)
}

type LTIResultRepository interface {
	Create(ctx context.Context, result *models.LTIResult) error
	FindByID(ctx context.Context, id uint) (*models.LTIResult, error)
	Upsert(ctx context.Context, result *models.LTIResult) error // Create or update by line_item_id + user_id
	ListByLineItem(ctx context.Context, lineItemID uint, params PaginationParams) (*PaginatedResult[models.LTIResult], error)
}
