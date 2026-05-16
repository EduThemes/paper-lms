package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAssignmentRepository mocks repository.AssignmentRepository
type MockAssignmentRepository struct {
	mock.Mock
}

func (m *MockAssignmentRepository) Create(ctx context.Context, assignment *models.Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockAssignmentRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Assignment, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Assignment), args.Error(1)
}

func (m *MockAssignmentRepository) Update(ctx context.Context, assignment *models.Assignment) error {
	args := m.Called(ctx, assignment)
	return args.Error(0)
}

func (m *MockAssignmentRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAssignmentRepository) ListByCourseID(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Assignment], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Assignment]), args.Error(1)
}

func (m *MockAssignmentRepository) FindByIDs(ctx context.Context, ids []uint) ([]models.Assignment, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Assignment), args.Error(1)
}
