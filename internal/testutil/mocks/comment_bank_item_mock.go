package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockCommentBankItemRepository mocks repository.CommentBankItemRepository.
// Delete accepts accountID per F-012 — test setups MUST stub with the
// matching accountID value (or use mock.AnythingOfType("uint")) so the
// cross-tenant path surfaces as a zero-rows-affected outcome.
type MockCommentBankItemRepository struct {
	mock.Mock
}

func (m *MockCommentBankItemRepository) Create(ctx context.Context, item *models.CommentBankItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockCommentBankItemRepository) FindByID(ctx context.Context, id uint) (*models.CommentBankItem, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CommentBankItem), args.Error(1)
}

func (m *MockCommentBankItemRepository) Update(ctx context.Context, item *models.CommentBankItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockCommentBankItemRepository) Delete(ctx context.Context, id, accountID uint) error {
	args := m.Called(ctx, id, accountID)
	return args.Error(0)
}

func (m *MockCommentBankItemRepository) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.CommentBankItem], error) {
	args := m.Called(ctx, userID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.CommentBankItem]), args.Error(1)
}

func (m *MockCommentBankItemRepository) SearchByUser(ctx context.Context, userID uint, query string) ([]models.CommentBankItem, error) {
	args := m.Called(ctx, userID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.CommentBankItem), args.Error(1)
}
