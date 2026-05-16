package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockSharedContentRepository mocks repository.SharedContentRepository
type MockSharedContentRepository struct {
	mock.Mock
}

func (m *MockSharedContentRepository) Create(ctx context.Context, item *models.SharedContent) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSharedContentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.SharedContent, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SharedContent), args.Error(1)
}

func (m *MockSharedContentRepository) Update(ctx context.Context, item *models.SharedContent) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockSharedContentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSharedContentRepository) ListByAccount(ctx context.Context, accountID uint, filters repository.SharedContentFilters, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	args := m.Called(ctx, accountID, filters, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.SharedContent]), args.Error(1)
}

func (m *MockSharedContentRepository) IncrementDownloadCount(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSharedContentRepository) ToggleFavorite(ctx context.Context, sharedContentID, userID uint) (bool, error) {
	args := m.Called(ctx, sharedContentID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockSharedContentRepository) IsFavorited(ctx context.Context, sharedContentID, userID uint) (bool, error) {
	args := m.Called(ctx, sharedContentID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *MockSharedContentRepository) ListUserFavorites(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.SharedContent], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.SharedContent]), args.Error(1)
}
