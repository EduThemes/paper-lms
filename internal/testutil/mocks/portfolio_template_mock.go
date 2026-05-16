package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockPortfolioTemplateRepository mocks repository.PortfolioTemplateRepository
type MockPortfolioTemplateRepository struct {
	mock.Mock
}

func (m *MockPortfolioTemplateRepository) Create(ctx context.Context, template *models.PortfolioTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockPortfolioTemplateRepository) FindByID(ctx context.Context, id, accountID uint) (*models.PortfolioTemplate, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PortfolioTemplate), args.Error(1)
}

func (m *MockPortfolioTemplateRepository) Update(ctx context.Context, template *models.PortfolioTemplate) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockPortfolioTemplateRepository) ListPublic(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioTemplate], error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PortfolioTemplate]), args.Error(1)
}

func (m *MockPortfolioTemplateRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.PortfolioTemplate], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.PortfolioTemplate]), args.Error(1)
}
