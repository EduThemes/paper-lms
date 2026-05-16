package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockOutcomeAlignmentRepository mocks repository.OutcomeAlignmentRepository.
type MockOutcomeAlignmentRepository struct {
	mock.Mock
}

func (m *MockOutcomeAlignmentRepository) Create(ctx context.Context, alignment *models.OutcomeAlignment) error {
	return m.Called(ctx, alignment).Error(0)
}

func (m *MockOutcomeAlignmentRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockOutcomeAlignmentRepository) ListByAssignmentID(ctx context.Context, assignmentID, accountID uint) ([]models.OutcomeAlignment, error) {
	args := m.Called(ctx, assignmentID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OutcomeAlignment), args.Error(1)
}

func (m *MockOutcomeAlignmentRepository) ListByCourseID(ctx context.Context, courseID, accountID uint) ([]models.OutcomeAlignment, error) {
	args := m.Called(ctx, courseID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OutcomeAlignment), args.Error(1)
}
