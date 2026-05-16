package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockConferenceRepository mocks repository.ConferenceRepository
type MockConferenceRepository struct {
	mock.Mock
}

func (m *MockConferenceRepository) Create(ctx context.Context, conference *models.Conference) error {
	args := m.Called(ctx, conference)
	return args.Error(0)
}

func (m *MockConferenceRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Conference, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conference), args.Error(1)
}

func (m *MockConferenceRepository) Update(ctx context.Context, conference *models.Conference) error {
	args := m.Called(ctx, conference)
	return args.Error(0)
}

func (m *MockConferenceRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConferenceRepository) ListByContext(ctx context.Context, contextType string, contextID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conference], error) {
	args := m.Called(ctx, contextType, contextID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Conference]), args.Error(1)
}
