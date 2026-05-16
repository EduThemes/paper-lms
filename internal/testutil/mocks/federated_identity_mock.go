package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockFederatedIdentityRepository mocks the Phase 9-PRE federated
// identity repo. LoginPipeline tests use it to drive the "user found"
// vs "user not found" branches; SSO handler tests use it to confirm
// the binding is created on first login.
type MockFederatedIdentityRepository struct {
	mock.Mock
}

func (m *MockFederatedIdentityRepository) FindByProviderAndSubject(ctx context.Context, providerID uint, externalSubject string) (*models.FederatedIdentity, error) {
	args := m.Called(ctx, providerID, externalSubject)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.FederatedIdentity), args.Error(1)
}

func (m *MockFederatedIdentityRepository) Create(ctx context.Context, fi *models.FederatedIdentity) error {
	args := m.Called(ctx, fi)
	return args.Error(0)
}

func (m *MockFederatedIdentityRepository) TouchLastSeen(ctx context.Context, id uint, claimsSnapshot []byte) error {
	args := m.Called(ctx, id, claimsSnapshot)
	return args.Error(0)
}

func (m *MockFederatedIdentityRepository) ListForUser(ctx context.Context, userID uint) ([]models.FederatedIdentity, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.FederatedIdentity), args.Error(1)
}
