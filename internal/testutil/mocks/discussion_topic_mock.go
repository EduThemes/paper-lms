package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockDiscussionTopicRepository mocks repository.DiscussionTopicRepository
type MockDiscussionTopicRepository struct {
	mock.Mock
}

func (m *MockDiscussionTopicRepository) Create(ctx context.Context, topic *models.DiscussionTopic) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
}

func (m *MockDiscussionTopicRepository) FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionTopic, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DiscussionTopic), args.Error(1)
}

func (m *MockDiscussionTopicRepository) Update(ctx context.Context, topic *models.DiscussionTopic) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
}

func (m *MockDiscussionTopicRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDiscussionTopicRepository) ListByCourseID(ctx context.Context, courseID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DiscussionTopic], error) {
	args := m.Called(ctx, courseID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.DiscussionTopic]), args.Error(1)
}
