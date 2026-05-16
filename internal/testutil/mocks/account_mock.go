package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockAccountRepository mocks repository.AccountRepository
type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) Create(ctx context.Context, account *models.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Account), args.Error(1)
}

func (m *MockAccountRepository) Update(ctx context.Context, account *models.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *MockAccountRepository) List(ctx context.Context, params repository.PaginationParams) (*repository.PaginatedResult[models.Account], error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Account]), args.Error(1)
}
