package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// MockLTILineItemRepository mocks repository.LTILineItemRepository.
//
// F-012: Delete takes accountID. accountID==0 means "no tenant scope"
// (auth-internal background callers). Handler-routed callers MUST pass
// callerAccountID(c).
type MockLTILineItemRepository struct {
	mock.Mock
}

func (m *MockLTILineItemRepository) Create(ctx context.Context, item *models.LTILineItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockLTILineItemRepository) FindByID(ctx context.Context, id uint) (*models.LTILineItem, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTILineItem), args.Error(1)
}

func (m *MockLTILineItemRepository) Update(ctx context.Context, item *models.LTILineItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockLTILineItemRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}

func (m *MockLTILineItemRepository) ListByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.LTILineItem], error) {
	args := m.Called(ctx, courseID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.LTILineItem]), args.Error(1)
}

func (m *MockLTILineItemRepository) FindByAssignmentID(ctx context.Context, assignmentID uint) (*models.LTILineItem, error) {
	args := m.Called(ctx, assignmentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTILineItem), args.Error(1)
}
