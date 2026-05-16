package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockOneRosterConnectionRepository mocks repository.OneRosterConnectionRepository
type MockOneRosterConnectionRepository struct {
	mock.Mock
}

func (m *MockOneRosterConnectionRepository) Create(ctx context.Context, conn *models.OneRosterConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockOneRosterConnectionRepository) FindByID(ctx context.Context, id, accountID uint) (*models.OneRosterConnection, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OneRosterConnection), args.Error(1)
}

func (m *MockOneRosterConnectionRepository) Update(ctx context.Context, conn *models.OneRosterConnection) error {
	args := m.Called(ctx, conn)
	return args.Error(0)
}

func (m *MockOneRosterConnectionRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOneRosterConnectionRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.OneRosterConnection], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.OneRosterConnection]), args.Error(1)
}

func (m *MockOneRosterConnectionRepository) FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.OneRosterConnection, error) {
	args := m.Called(ctx, accountID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OneRosterConnection), args.Error(1)
}

func (m *MockOneRosterConnectionRepository) ListAutoSync(ctx context.Context) ([]models.OneRosterConnection, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.OneRosterConnection), args.Error(1)
}
