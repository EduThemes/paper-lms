package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// MockSettingRepository mocks repository.SettingRepository.
type MockSettingRepository struct {
	mock.Mock
}

func (m *MockSettingRepository) FindByScope(ctx context.Context, scopeType string, scopeID uint, key string) (*models.Setting, error) {
	args := m.Called(ctx, scopeType, scopeID, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Setting), args.Error(1)
}

func (m *MockSettingRepository) ListByScope(ctx context.Context, scopeType string, scopeID uint) ([]models.Setting, error) {
	args := m.Called(ctx, scopeType, scopeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Setting), args.Error(1)
}

func (m *MockSettingRepository) Upsert(ctx context.Context, setting *models.Setting) error {
	args := m.Called(ctx, setting)
	return args.Error(0)
}

func (m *MockSettingRepository) Delete(ctx context.Context, scopeType string, scopeID uint, key string) error {
	args := m.Called(ctx, scopeType, scopeID, key)
	return args.Error(0)
}
