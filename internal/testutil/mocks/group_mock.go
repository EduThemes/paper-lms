package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockGroupRepository implements repository.GroupRepository for testing.
type MockGroupRepository struct {
	mock.Mock
}

func (m *MockGroupRepository) Create(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Group, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupRepository) Update(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupRepository) ListByCategoryID(ctx context.Context, categoryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	args := m.Called(ctx, categoryID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Group]), args.Error(1)
}

func (m *MockGroupRepository) ListByContextID(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	args := m.Called(ctx, contextType, contextID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Group]), args.Error(1)
}

func (m *MockGroupRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Group]), args.Error(1)
}
