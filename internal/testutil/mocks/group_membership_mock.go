package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockGroupMembershipRepository implements repository.GroupMembershipRepository for testing.
type MockGroupMembershipRepository struct {
	mock.Mock
}

func (m *MockGroupMembershipRepository) Create(ctx context.Context, membership *models.GroupMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) FindByID(ctx context.Context, id, accountID uint) (*models.GroupMembership, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupMembership), args.Error(1)
}

func (m *MockGroupMembershipRepository) Update(ctx context.Context, membership *models.GroupMembership) error {
	args := m.Called(ctx, membership)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupMembershipRepository) ListByGroupID(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupMembership], error) {
	args := m.Called(ctx, groupID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.GroupMembership]), args.Error(1)
}

func (m *MockGroupMembershipRepository) FindByGroupAndUser(ctx context.Context, groupID, userID uint) (*models.GroupMembership, error) {
	args := m.Called(ctx, groupID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.GroupMembership), args.Error(1)
}

func (m *MockGroupMembershipRepository) FindUserGroupInCategory(ctx context.Context, userID, groupCategoryID uint) (*models.Group, error) {
	args := m.Called(ctx, userID, groupCategoryID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}
