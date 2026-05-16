package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/stretchr/testify/mock"
)

// MockDiscussionEntryParticipantRepository mocks repository.DiscussionEntryParticipantRepository
type MockDiscussionEntryParticipantRepository struct {
	mock.Mock
}

func (m *MockDiscussionEntryParticipantRepository) Create(ctx context.Context, p *models.DiscussionEntryParticipant) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockDiscussionEntryParticipantRepository) FindByEntryAndUser(ctx context.Context, entryID, userID, accountID uint) (*models.DiscussionEntryParticipant, error) {
	args := m.Called(ctx, entryID, userID, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DiscussionEntryParticipant), args.Error(1)
}

func (m *MockDiscussionEntryParticipantRepository) MarkAsRead(ctx context.Context, entryID, userID uint) error {
	args := m.Called(ctx, entryID, userID)
	return args.Error(0)
}

func (m *MockDiscussionEntryParticipantRepository) MarkTopicAsRead(ctx context.Context, topicID, userID uint) error {
	args := m.Called(ctx, topicID, userID)
	return args.Error(0)
}

func (m *MockDiscussionEntryParticipantRepository) CountUnreadByTopic(ctx context.Context, topicID, userID uint) (int64, error) {
	args := m.Called(ctx, topicID, userID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDiscussionEntryParticipantRepository) ListUnreadByTopic(ctx context.Context, topicID, userID uint) ([]uint, error) {
	args := m.Called(ctx, topicID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uint), args.Error(1)
}
