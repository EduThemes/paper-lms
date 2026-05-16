package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockGradingPeriodGroupRepository mocks repository.GradingPeriodGroupRepository
type MockGradingPeriodGroupRepository struct {
	mock.Mock
}

func (m *MockGradingPeriodGroupRepository) Create(ctx context.Context, group *models.GradingPeriodGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriodGroup, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingPeriodGroup), args.Error(1)
}

func (m *MockGradingPeriodGroupRepository) Update(ctx context.Context, group *models.GradingPeriodGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingPeriodGroupRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GradingPeriodGroup], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GradingPeriodGroup]), args.Error(1)
}
