package mocks

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockModuleRepository mocks repository.ModuleRepository
type MockModuleRepository struct {
	mock.Mock
}

func (m *MockModuleRepository) Create(ctx context.Context, module *models.ContextModule) error {
	args := m.Called(ctx, module)
	return args.Error(0)
}

func (m *MockModuleRepository) FindByID(ctx context.Context, id, accountID uint) (*models.ContextModule, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContextModule), args.Error(1)
}

func (m *MockModuleRepository) Update(ctx context.Context, module *models.ContextModule) error {
	args := m.Called(ctx, module)
	return args.Error(0)
}

func (m *MockModuleRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockModuleRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ContextModule], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ContextModule]), args.Error(1)
}

func (m *MockModuleRepository) FindActiveByDateRange(ctx context.Context, courseID uint, date time.Time) (*models.ContextModule, error) {
	args := m.Called(ctx, courseID, date)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ContextModule), args.Error(1)
}

func (m *MockModuleRepository) ReorderModules(ctx context.Context, courseID uint, moduleIDs []uint) error {
	args := m.Called(ctx, courseID, moduleIDs)
	return args.Error(0)
}
