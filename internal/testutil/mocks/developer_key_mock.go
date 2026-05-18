package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockDeveloperKeyRepository mocks repository.DeveloperKeyRepository.
//
// 13.1.D Wave 2: FindByID and Delete take accountID. accountID==0 means
// "no tenant scope" (internal background callers — OAuth2 token exchange,
// LTI launch-token build). Handler-routed callers MUST pass
// callerAccountID(c). FindByClientID stays 1-arg because client_id is the
// OAuth2/LTI external entry-point lookup key and the tenant is not
// pre-known at that point.
type MockDeveloperKeyRepository struct {
	mock.Mock
}

func (m *MockDeveloperKeyRepository) Create(ctx context.Context, key *models.DeveloperKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockDeveloperKeyRepository) FindByID(ctx context.Context, id, accountID uint) (*models.DeveloperKey, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeveloperKey), args.Error(1)
}

func (m *MockDeveloperKeyRepository) FindByClientID(ctx context.Context, clientID string) (*models.DeveloperKey, error) {
	args := m.Called(ctx, clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeveloperKey), args.Error(1)
}

func (m *MockDeveloperKeyRepository) Update(ctx context.Context, key *models.DeveloperKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockDeveloperKeyRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}

func (m *MockDeveloperKeyRepository) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DeveloperKey], error) {
	args := m.Called(ctx, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.DeveloperKey]), args.Error(1)
}
