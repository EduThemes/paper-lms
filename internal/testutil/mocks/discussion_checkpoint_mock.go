package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockDiscussionCheckpointRepository mocks repository.DiscussionCheckpointRepository
type MockDiscussionCheckpointRepository struct {
	mock.Mock
}

func (m *MockDiscussionCheckpointRepository) Create(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockDiscussionCheckpointRepository) FindByID(ctx context.Context, id, accountID uint) (*models.DiscussionCheckpoint, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DiscussionCheckpoint), args.Error(1)
}

func (m *MockDiscussionCheckpointRepository) Update(ctx context.Context, checkpoint *models.DiscussionCheckpoint) error {
	args := m.Called(ctx, checkpoint)
	return args.Error(0)
}

func (m *MockDiscussionCheckpointRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDiscussionCheckpointRepository) ListByTopicID(ctx context.Context, topicID, accountID uint) ([]models.DiscussionCheckpoint, error) {
	args := m.Called(ctx, topicID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.DiscussionCheckpoint), args.Error(1)
}

func (m *MockDiscussionCheckpointRepository) DeleteByTopicID(ctx context.Context, topicID uint) error {
	args := m.Called(ctx, topicID)
	return args.Error(0)
}
