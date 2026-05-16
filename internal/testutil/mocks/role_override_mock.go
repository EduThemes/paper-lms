package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockRoleOverrideRepository mocks repository.RoleOverrideRepository
type MockRoleOverrideRepository struct {
	mock.Mock
}

func (m *MockRoleOverrideRepository) Create(ctx context.Context, override *models.RoleOverride) error {
	args := m.Called(ctx, override)
	return args.Error(0)
}

func (m *MockRoleOverrideRepository) FindByID(ctx context.Context, id, accountID uint) (*models.RoleOverride, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RoleOverride), args.Error(1)
}

func (m *MockRoleOverrideRepository) Update(ctx context.Context, override *models.RoleOverride) error {
	args := m.Called(ctx, override)
	return args.Error(0)
}

func (m *MockRoleOverrideRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoleOverrideRepository) ListByRoleID(ctx context.Context, roleID uint) ([]models.RoleOverride, error) {
	args := m.Called(ctx, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.RoleOverride), args.Error(1)
}

func (m *MockRoleOverrideRepository) FindByRoleAndPermission(ctx context.Context, roleID uint, permission string) (*models.RoleOverride, error) {
	args := m.Called(ctx, roleID, permission)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RoleOverride), args.Error(1)
}

func (m *MockRoleOverrideRepository) ListByAccountID(ctx context.Context, accountID uint) ([]models.RoleOverride, error) {
	args := m.Called(ctx, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.RoleOverride), args.Error(1)
}

func (m *MockRoleOverrideRepository) BulkUpsert(ctx context.Context, overrides []models.RoleOverride) error {
	args := m.Called(ctx, overrides)
	return args.Error(0)
}
