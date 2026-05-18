package mocks

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/stretchr/testify/mock"
)

// MockConversationRepository mocks repository.ConversationRepository
type MockConversationRepository struct {
	mock.Mock
}

func (m *MockConversationRepository) Create(ctx context.Context, conversation *models.Conversation) error {
	args := m.Called(ctx, conversation)
	return args.Error(0)
}

func (m *MockConversationRepository) FindByID(ctx context.Context, id, accountID uint) (*models.Conversation, error) {
	args := m.Called(ctx, id, accountID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Conversation), args.Error(1)
}

func (m *MockConversationRepository) Update(ctx context.Context, conversation *models.Conversation) error {
	args := m.Called(ctx, conversation)
	return args.Error(0)
}

func (m *MockConversationRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConversationRepository) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conversation], error) {
	args := m.Called(ctx, userID, accountID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.Conversation]), args.Error(1)
}

// MockConversationParticipantRepository mocks repository.ConversationParticipantRepository
type MockConversationParticipantRepository struct {
	mock.Mock
}

func (m *MockConversationParticipantRepository) Create(ctx context.Context, participant *models.ConversationParticipant) error {
	args := m.Called(ctx, participant)
	return args.Error(0)
}

func (m *MockConversationParticipantRepository) FindByConversationAndUser(ctx context.Context, conversationID, userID uint) (*models.ConversationParticipant, error) {
	args := m.Called(ctx, conversationID, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationParticipant), args.Error(1)
}

func (m *MockConversationParticipantRepository) Update(ctx context.Context, participant *models.ConversationParticipant) error {
	args := m.Called(ctx, participant)
	return args.Error(0)
}

func (m *MockConversationParticipantRepository) Delete(ctx context.Context, conversationID, userID uint) error {
	args := m.Called(ctx, conversationID, userID)
	return args.Error(0)
}

func (m *MockConversationParticipantRepository) ListByConversationID(ctx context.Context, conversationID uint) ([]models.ConversationParticipant, error) {
	args := m.Called(ctx, conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConversationParticipant), args.Error(1)
}

func (m *MockConversationParticipantRepository) ListByUserID(ctx context.Context, userID uint) ([]models.ConversationParticipant, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConversationParticipant), args.Error(1)
}

// MockConversationMessageRepository mocks repository.ConversationMessageRepository
type MockConversationMessageRepository struct {
	mock.Mock
}

func (m *MockConversationMessageRepository) Create(ctx context.Context, message *models.ConversationMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockConversationMessageRepository) FindByID(ctx context.Context, id uint) (*models.ConversationMessage, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ConversationMessage), args.Error(1)
}

func (m *MockConversationMessageRepository) Update(ctx context.Context, message *models.ConversationMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *MockConversationMessageRepository) Delete(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockConversationMessageRepository) ListByConversationID(ctx context.Context, conversationID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ConversationMessage], error) {
	args := m.Called(ctx, conversationID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.PaginatedResult[models.ConversationMessage]), args.Error(1)
}
