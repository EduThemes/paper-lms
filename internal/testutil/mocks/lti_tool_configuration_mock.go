package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// MockLTIToolConfigurationRepository mocks
// repository.LTIToolConfigurationRepository.
//
// F-012: Delete takes accountID. accountID==0 means "no tenant scope"
// (auth-internal background callers).
type MockLTIToolConfigurationRepository struct {
	mock.Mock
}

func (m *MockLTIToolConfigurationRepository) Create(ctx context.Context, config *models.LTIToolConfiguration) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockLTIToolConfigurationRepository) FindByID(ctx context.Context, id uint) (*models.LTIToolConfiguration, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTIToolConfiguration), args.Error(1)
}

func (m *MockLTIToolConfigurationRepository) FindByDeveloperKeyID(ctx context.Context, devKeyID uint) (*models.LTIToolConfiguration, error) {
	args := m.Called(ctx, devKeyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.LTIToolConfiguration), args.Error(1)
}

func (m *MockLTIToolConfigurationRepository) Update(ctx context.Context, config *models.LTIToolConfiguration) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

func (m *MockLTIToolConfigurationRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}
