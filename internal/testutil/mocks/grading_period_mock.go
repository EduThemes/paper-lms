package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockGradingPeriodRepository mocks repository.GradingPeriodRepository
type MockGradingPeriodRepository struct {
	mock.Mock
}

func (m *MockGradingPeriodRepository) Create(ctx context.Context, period *models.GradingPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) FindByID(ctx context.Context, id, accountID uint) (*models.GradingPeriod, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingPeriod), args.Error(1)
}

func (m *MockGradingPeriodRepository) Update(ctx context.Context, period *models.GradingPeriod) error {
	args := m.Called(ctx, period)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingPeriodRepository) ListByGroupID(ctx context.Context, groupID, accountID uint) ([]models.GradingPeriod, error) {
	args := m.Called(ctx, groupID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GradingPeriod), args.Error(1)
}
