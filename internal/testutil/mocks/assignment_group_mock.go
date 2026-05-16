package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAssignmentGroupRepository mocks repository.AssignmentGroupRepository
type MockAssignmentGroupRepository struct {
	mock.Mock
}

func (m *MockAssignmentGroupRepository) Create(ctx context.Context, group *models.AssignmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) FindByID(ctx context.Context, id uint) (*models.AssignmentGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AssignmentGroup), args.Error(1)
}

func (m *MockAssignmentGroupRepository) Update(ctx context.Context, group *models.AssignmentGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAssignmentGroupRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AssignmentGroup], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.AssignmentGroup]), args.Error(1)
}
