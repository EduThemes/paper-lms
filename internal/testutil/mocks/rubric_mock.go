package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockRubricRepository mocks repository.RubricRepository.
type MockRubricRepository struct {
	mock.Mock
}

func (m *MockRubricRepository) Create(ctx context.Context, rubric *models.Rubric) error {
	return m.Called(ctx, rubric).Error(0)
}

func (m *MockRubricRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Rubric, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Rubric), args.Error(1)
}

func (m *MockRubricRepository) Update(ctx context.Context, rubric *models.Rubric) error {
	return m.Called(ctx, rubric).Error(0)
}

func (m *MockRubricRepository) Delete(ctx context.Context, id, accountID uint) error {
	return m.Called(ctx, id, accountID).Error(0)
}

func (m *MockRubricRepository) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Rubric], error) {
	args := m.Called(ctx, contextType, contextID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Rubric]), args.Error(1)
}

// MockRubricAssociationRepository mocks repository.RubricAssociationRepository.
type MockRubricAssociationRepository struct {
	mock.Mock
}

func (m *MockRubricAssociationRepository) Create(ctx context.Context, assoc *models.RubricAssociation) error {
	return m.Called(ctx, assoc).Error(0)
}

func (m *MockRubricAssociationRepository) FindByID(ctx context.Context, id uint) (*models.RubricAssociation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RubricAssociation), args.Error(1)
}

func (m *MockRubricAssociationRepository) Update(ctx context.Context, assoc *models.RubricAssociation) error {
	return m.Called(ctx, assoc).Error(0)
}

func (m *MockRubricAssociationRepository) Delete(ctx context.Context, id uint) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockRubricAssociationRepository) FindByAssociation(ctx context.Context, associationID uint, associationType string) (*models.RubricAssociation, error) {
	args := m.Called(ctx, associationID, associationType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RubricAssociation), args.Error(1)
}

// MockRubricAssessmentRepository mocks repository.RubricAssessmentRepository.
type MockRubricAssessmentRepository struct {
	mock.Mock
}

func (m *MockRubricAssessmentRepository) Create(ctx context.Context, assessment *models.RubricAssessment) error {
	return m.Called(ctx, assessment).Error(0)
}

func (m *MockRubricAssessmentRepository) FindByID(ctx context.Context, id uint) (*models.RubricAssessment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RubricAssessment), args.Error(1)
}

func (m *MockRubricAssessmentRepository) Update(ctx context.Context, assessment *models.RubricAssessment) error {
	return m.Called(ctx, assessment).Error(0)
}

func (m *MockRubricAssessmentRepository) Delete(ctx context.Context, id, accountID uint) error {
	return m.Called(ctx, id, accountID).Error(0)
}

func (m *MockRubricAssessmentRepository) FindByUserAndAssociation(ctx context.Context, userID, assessorID, rubricAssocID uint) (*models.RubricAssessment, error) {
	args := m.Called(ctx, userID, assessorID, rubricAssocID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RubricAssessment), args.Error(1)
}

func (m *MockRubricAssessmentRepository) ListByAssociationID(ctx context.Context, rubricAssocID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.RubricAssessment], error) {
	args := m.Called(ctx, rubricAssocID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.RubricAssessment]), args.Error(1)
}
