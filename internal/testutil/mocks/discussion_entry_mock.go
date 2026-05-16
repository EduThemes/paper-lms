package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockDiscussionEntryRepository mocks repository.DiscussionEntryRepository
type MockDiscussionEntryRepository struct {
	mock.Mock
}

func (m *MockDiscussionEntryRepository) Create(ctx context.Context, entry *models.DiscussionEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockDiscussionEntryRepository) FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionEntry, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DiscussionEntry), args.Error(1)
}

func (m *MockDiscussionEntryRepository) Update(ctx context.Context, entry *models.DiscussionEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockDiscussionEntryRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDiscussionEntryRepository) ListByTopicID(ctx context.Context, topicID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	args := m.Called(ctx, topicID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.DiscussionEntry]), args.Error(1)
}

func (m *MockDiscussionEntryRepository) ListReplies(ctx context.Context, entryID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionEntry], error) {
	args := m.Called(ctx, entryID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.DiscussionEntry]), args.Error(1)
}

func (m *MockDiscussionEntryRepository) ListAllByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionEntry, error) {
	args := m.Called(ctx, topicID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DiscussionEntry), args.Error(1)
}
