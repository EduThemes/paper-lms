package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockModuleItemRepository mocks repository.ModuleItemRepository
type MockModuleItemRepository struct {
	mock.Mock
}

func (m *MockModuleItemRepository) Create(ctx context.Context, item *models.ContentTag) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockModuleItemRepository) FindByID(ctx context.Context, id uint) (*models.ContentTag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContentTag), args.Error(1)
}

func (m *MockModuleItemRepository) Update(ctx context.Context, item *models.ContentTag) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockModuleItemRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockModuleItemRepository) ListByModuleID(ctx context.Context, moduleID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContentTag], error) {
	args := m.Called(ctx, moduleID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ContentTag]), args.Error(1)
}

func (m *MockModuleItemRepository) ReorderItems(ctx context.Context, moduleID uint, itemIDs []uint) error {
	args := m.Called(ctx, moduleID, itemIDs)
	return args.Error(0)
}

func (m *MockModuleItemRepository) MoveItemToModule(ctx context.Context, itemID uint, targetModuleID uint, position int) error {
	args := m.Called(ctx, itemID, targetModuleID, position)
	return args.Error(0)
}
