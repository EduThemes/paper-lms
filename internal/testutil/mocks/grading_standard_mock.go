package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockGradingStandardRepository mocks repository.GradingStandardRepository
type MockGradingStandardRepository struct {
	mock.Mock
}

func (m *MockGradingStandardRepository) Create(ctx context.Context, standard *models.GradingStandard) error {
	args := m.Called(ctx, standard)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) FindByID(ctx context.Context, id uint) (*models.GradingStandard, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingStandard), args.Error(1)
}

func (m *MockGradingStandardRepository) Update(ctx context.Context, standard *models.GradingStandard) error {
	args := m.Called(ctx, standard)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGradingStandardRepository) ListByCourse(ctx context.Context, courseID uint) ([]models.GradingStandard, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.GradingStandard), args.Error(1)
}

func (m *MockGradingStandardRepository) FindActiveByCourse(ctx context.Context, courseID uint) (*models.GradingStandard, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GradingStandard), args.Error(1)
}
