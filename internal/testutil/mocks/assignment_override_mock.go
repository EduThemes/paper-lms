package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockAssignmentOverrideRepository mocks repository.AssignmentOverrideRepository.
type MockAssignmentOverrideRepository struct {
	mock.Mock
}

func (m *MockAssignmentOverrideRepository) Create(ctx context.Context, override *models.AssignmentOverride) error {
	args := m.Called(ctx, override)
	return args.Error(0)
}

func (m *MockAssignmentOverrideRepository) FindByID(ctx context.Context, id uint) (*models.AssignmentOverride, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AssignmentOverride), args.Error(1)
}

func (m *MockAssignmentOverrideRepository) Update(ctx context.Context, override *models.AssignmentOverride) error {
	args := m.Called(ctx, override)
	return args.Error(0)
}

func (m *MockAssignmentOverrideRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAssignmentOverrideRepository) ListByAssignmentID(ctx context.Context, assignmentID uint) ([]models.AssignmentOverride, error) {
	args := m.Called(ctx, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AssignmentOverride), args.Error(1)
}

// MockAssignmentOverrideStudentRepository mocks repository.AssignmentOverrideStudentRepository.
type MockAssignmentOverrideStudentRepository struct {
	mock.Mock
}

func (m *MockAssignmentOverrideStudentRepository) Create(ctx context.Context, student *models.AssignmentOverrideStudent) error {
	args := m.Called(ctx, student)
	return args.Error(0)
}

func (m *MockAssignmentOverrideStudentRepository) Delete(ctx context.Context, overrideID, userID uint) error {
	args := m.Called(ctx, overrideID, userID)
	return args.Error(0)
}

func (m *MockAssignmentOverrideStudentRepository) ListByOverrideID(ctx context.Context, overrideID uint) ([]models.AssignmentOverrideStudent, error) {
	args := m.Called(ctx, overrideID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AssignmentOverrideStudent), args.Error(1)
}

func (m *MockAssignmentOverrideStudentRepository) ListByUserAndAssignment(ctx context.Context, userID, assignmentID uint) ([]models.AssignmentOverrideStudent, error) {
	args := m.Called(ctx, userID, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AssignmentOverrideStudent), args.Error(1)
}
