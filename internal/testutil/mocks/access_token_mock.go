package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAccessTokenRepository mocks repository.AccessTokenRepository
type MockAccessTokenRepository struct {
	mock.Mock
}

func (m *MockAccessTokenRepository) Create(ctx context.Context, token *models.AccessToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) FindByID(ctx context.Context, id uint) (*models.AccessToken, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error) {
	args := m.Called(ctx, tokenHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) FindByRefreshToken(ctx context.Context, refreshToken string) (*models.AccessToken, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.AccessToken), args.Error(1)
}

func (m *MockAccessTokenRepository) Update(ctx context.Context, token *models.AccessToken) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAccessTokenRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AccessToken], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.AccessToken]), args.Error(1)
}

func (m *MockAccessTokenRepository) DeleteExpired(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
