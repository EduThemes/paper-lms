package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockGroupCategoryRepository implements repository.GroupCategoryRepository for testing.
type MockGroupCategoryRepository struct {
	mock.Mock
}

func (m *MockGroupCategoryRepository) Create(ctx context.Context, category *models.GroupCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockGroupCategoryRepository) FindByID(ctx context.Context, id, accountID uint) (*models.GroupCategory, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupCategory), args.Error(1)
}

func (m *MockGroupCategoryRepository) Update(ctx context.Context, category *models.GroupCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockGroupCategoryRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupCategoryRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GroupCategory]), args.Error(1)
}

func (m *MockGroupCategoryRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GroupCategory]), args.Error(1)
}
