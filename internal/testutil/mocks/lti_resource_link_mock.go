package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// MockLTIResourceLinkRepository mocks repository.LTIResourceLinkRepository.
//
// F-012: Delete takes accountID. accountID==0 means "no tenant scope"
// (auth-internal background callers).
type MockLTIResourceLinkRepository struct {
	mock.Mock
}

func (m *MockLTIResourceLinkRepository) Create(ctx context.Context, link *models.LTIResourceLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockLTIResourceLinkRepository) FindByID(ctx context.Context, id uint) (*models.LTIResourceLink, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTIResourceLink), args.Error(1)
}

func (m *MockLTIResourceLinkRepository) FindByResourceLinkID(ctx context.Context, resourceLinkID string) (*models.LTIResourceLink, error) {
	args := m.Called(ctx, resourceLinkID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTIResourceLink), args.Error(1)
}

func (m *MockLTIResourceLinkRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}
