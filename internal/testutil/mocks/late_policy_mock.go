package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockLatePolicyRepository mocks repository.LatePolicyRepository
type MockLatePolicyRepository struct {
	mock.Mock
}

func (m *MockLatePolicyRepository) Create(ctx context.Context, policy *models.LatePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockLatePolicyRepository) FindByCourseID(ctx context.Context, courseID uint) (*models.LatePolicy, error) {
	args := m.Called(ctx, courseID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LatePolicy), args.Error(1)
}

func (m *MockLatePolicyRepository) Update(ctx context.Context, policy *models.LatePolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}

func (m *MockLatePolicyRepository) Delete(ctx context.Context, courseID uint) error {
	args := m.Called(ctx, courseID)
	return args.Error(0)
}
