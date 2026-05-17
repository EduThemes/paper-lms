package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type PortfolioRepository interface {
	Create(ctx context.Context, portfolio *models.Portfolio) error
	FindByID(ctx context.Context, id uint) (*models.Portfolio, error)
	FindBySlug(ctx context.Context, slug string) (*models.Portfolio, error)
	FindByPublicURL(ctx context.Context, publicURL string) (*models.Portfolio, error)
	Update(ctx context.Context, portfolio *models.Portfolio) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Portfolio], error)
	ListPublic(ctx context.Context, params PaginationParams) (*PaginatedResult[models.Portfolio], error)
	IncrementViewCount(ctx context.Context, id uint) error
}

type PortfolioSectionRepository interface {
	Create(ctx context.Context, section *models.PortfolioSection) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioSection, error)
	FindByIDs(ctx context.Context, ids []uint) ([]models.PortfolioSection, error)
	Update(ctx context.Context, section *models.PortfolioSection) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, error)
}

type PortfolioArtifactRepository interface {
	Create(ctx context.Context, artifact *models.PortfolioArtifact) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioArtifact, error)
	Update(ctx context.Context, artifact *models.PortfolioArtifact) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint, params PaginationParams) (*PaginatedResult[models.PortfolioArtifact], error)
	ListBySectionID(ctx context.Context, sectionID uint) ([]models.PortfolioArtifact, error)
	ListFeatured(ctx context.Context, portfolioID uint) ([]models.PortfolioArtifact, error)
}

type PortfolioReflectionRepository interface {
	Create(ctx context.Context, reflection *models.PortfolioReflection) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioReflection, error)
	Update(ctx context.Context, reflection *models.PortfolioReflection) error
	ListByArtifactID(ctx context.Context, artifactID uint) ([]models.PortfolioReflection, error)
}

type PortfolioTemplateRepository interface {
	Create(ctx context.Context, template *models.PortfolioTemplate) error
	// 13.1.D — direct account_id column. Note: portfolio templates ARE
	// account-scoped (admin-curated). User portfolios live in
	// PortfolioRepository and stay user-scoped (private, owner-only).
	FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioTemplate, error)
	Update(ctx context.Context, template *models.PortfolioTemplate) error
	ListPublic(ctx context.Context, params PaginationParams) (*PaginatedResult[models.PortfolioTemplate], error)
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.PortfolioTemplate], error)
}

type PortfolioCommentRepository interface {
	Create(ctx context.Context, comment *models.PortfolioComment) error
	FindByID(ctx context.Context, id uint) (*models.PortfolioComment, error)
	Update(ctx context.Context, comment *models.PortfolioComment) error
	Delete(ctx context.Context, id uint) error
	ListByPortfolioID(ctx context.Context, portfolioID uint, params PaginationParams) (*PaginatedResult[models.PortfolioComment], error)
	ListByArtifactID(ctx context.Context, artifactID uint, params PaginationParams) (*PaginatedResult[models.PortfolioComment], error)
}
