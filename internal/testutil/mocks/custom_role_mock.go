package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCustomRoleRepository mocks repository.CustomRoleRepository
type MockCustomRoleRepository struct {
	mock.Mock
}

func (m *MockCustomRoleRepository) Create(ctx context.Context, role *models.CustomRole) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockCustomRoleRepository) FindByID(ctx context.Context, id, accountID uint) (*models.CustomRole, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CustomRole), args.Error(1)
}

func (m *MockCustomRoleRepository) Update(ctx context.Context, role *models.CustomRole) error {
	args := m.Called(ctx, role)
	return args.Error(0)
}

func (m *MockCustomRoleRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCustomRoleRepository) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CustomRole], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CustomRole]), args.Error(1)
}

func (m *MockCustomRoleRepository) FindByAccountAndName(ctx context.Context, accountID uint, name string) (*models.CustomRole, error) {
	args := m.Called(ctx, accountID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CustomRole), args.Error(1)
}

func (m *MockCustomRoleRepository) ListByBaseRoleType(ctx context.Context, accountID uint, baseRoleType string) ([]models.CustomRole, error) {
	args := m.Called(ctx, accountID, baseRoleType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CustomRole), args.Error(1)
}

func (m *MockCustomRoleRepository) ListActive(ctx context.Context, accountID uint) ([]models.CustomRole, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CustomRole), args.Error(1)
}
