package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPortfolioRepository mocks repository.PortfolioRepository
type MockPortfolioRepository struct {
	mock.Mock
}

func (m *MockPortfolioRepository) Create(ctx context.Context, portfolio *models.Portfolio) error {
	args := m.Called(ctx, portfolio)
	return args.Error(0)
}

func (m *MockPortfolioRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Portfolio, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) FindBySlug(ctx context.Context, slug string) (*models.Portfolio, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) FindByPublicURL(ctx context.Context, publicURL string) (*models.Portfolio, error) {
	args := m.Called(ctx, publicURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Portfolio), args.Error(1)
}

func (m *MockPortfolioRepository) Update(ctx context.Context, portfolio *models.Portfolio) error {
	args := m.Called(ctx, portfolio)
	return args.Error(0)
}

func (m *MockPortfolioRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Portfolio]), args.Error(1)
}

func (m *MockPortfolioRepository) ListPublic(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Portfolio], error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Portfolio]), args.Error(1)
}

func (m *MockPortfolioRepository) IncrementViewCount(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockPortfolioSectionRepository mocks repository.PortfolioSectionRepository
type MockPortfolioSectionRepository struct {
	mock.Mock
}

func (m *MockPortfolioSectionRepository) Create(ctx context.Context, section *models.PortfolioSection) error {
	args := m.Called(ctx, section)
	return args.Error(0)
}

func (m *MockPortfolioSectionRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioSection, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PortfolioSection), args.Error(1)
}

func (m *MockPortfolioSectionRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.PortfolioSection, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PortfolioSection), args.Error(1)
}

func (m *MockPortfolioSectionRepository) Update(ctx context.Context, section *models.PortfolioSection) error {
	args := m.Called(ctx, section)
	return args.Error(0)
}

func (m *MockPortfolioSectionRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioSectionRepository) ListByPortfolioID(ctx context.Context, portfolioID uint) ([]models.PortfolioSection, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PortfolioSection), args.Error(1)
}

// MockPortfolioArtifactRepository mocks repository.PortfolioArtifactRepository
type MockPortfolioArtifactRepository struct {
	mock.Mock
}

func (m *MockPortfolioArtifactRepository) Create(ctx context.Context, artifact *models.PortfolioArtifact) error {
	args := m.Called(ctx, artifact)
	return args.Error(0)
}

func (m *MockPortfolioArtifactRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioArtifact, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PortfolioArtifact), args.Error(1)
}

func (m *MockPortfolioArtifactRepository) Update(ctx context.Context, artifact *models.PortfolioArtifact) error {
	args := m.Called(ctx, artifact)
	return args.Error(0)
}

func (m *MockPortfolioArtifactRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioArtifactRepository) ListByPortfolioID(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioArtifact], error) {
	args := m.Called(ctx, portfolioID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PortfolioArtifact]), args.Error(1)
}

func (m *MockPortfolioArtifactRepository) ListBySectionID(ctx context.Context, sectionID uint) ([]models.PortfolioArtifact, error) {
	args := m.Called(ctx, sectionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PortfolioArtifact), args.Error(1)
}

func (m *MockPortfolioArtifactRepository) ListFeatured(ctx context.Context, portfolioID uint) ([]models.PortfolioArtifact, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PortfolioArtifact), args.Error(1)
}

// MockPortfolioReflectionRepository mocks repository.PortfolioReflectionRepository
type MockPortfolioReflectionRepository struct {
	mock.Mock
}

func (m *MockPortfolioReflectionRepository) Create(ctx context.Context, reflection *models.PortfolioReflection) error {
	args := m.Called(ctx, reflection)
	return args.Error(0)
}

func (m *MockPortfolioReflectionRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioReflection, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PortfolioReflection), args.Error(1)
}

func (m *MockPortfolioReflectionRepository) Update(ctx context.Context, reflection *models.PortfolioReflection) error {
	args := m.Called(ctx, reflection)
	return args.Error(0)
}

func (m *MockPortfolioReflectionRepository) ListByArtifactID(ctx context.Context, artifactID uint) ([]models.PortfolioReflection, error) {
	args := m.Called(ctx, artifactID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PortfolioReflection), args.Error(1)
}

// MockPortfolioCommentRepository mocks repository.PortfolioCommentRepository
type MockPortfolioCommentRepository struct {
	mock.Mock
}

func (m *MockPortfolioCommentRepository) Create(ctx context.Context, comment *models.PortfolioComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockPortfolioCommentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioComment, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PortfolioComment), args.Error(1)
}

func (m *MockPortfolioCommentRepository) Update(ctx context.Context, comment *models.PortfolioComment) error {
	args := m.Called(ctx, comment)
	return args.Error(0)
}

func (m *MockPortfolioCommentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPortfolioCommentRepository) ListByPortfolioID(ctx context.Context, portfolioID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	args := m.Called(ctx, portfolioID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PortfolioComment]), args.Error(1)
}

func (m *MockPortfolioCommentRepository) ListByArtifactID(ctx context.Context, artifactID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioComment], error) {
	args := m.Called(ctx, artifactID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PortfolioComment]), args.Error(1)
}
