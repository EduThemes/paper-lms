package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type AuthenticationProviderRepository interface {
	Create(ctx context.Context, provider *models.AuthenticationProvider) error
	FindByID(ctx context.Context, id uint) (*models.AuthenticationProvider, error)
	Update(ctx context.Context, provider *models.AuthenticationProvider) error
	Delete(ctx context.Context, id uint) error
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.AuthenticationProvider], error)
	FindByAccountAndType(ctx context.Context, accountID uint, authType string) ([]models.AuthenticationProvider, error)
}
