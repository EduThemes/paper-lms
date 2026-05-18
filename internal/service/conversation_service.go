package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type ConversationService struct {
	convRepo        repository.ConversationRepository
	participantRepo repository.ConversationParticipantRepository
	messageRepo     repository.ConversationMessageRepository
}

func NewConversationService(
	convRepo repository.ConversationRepository,
	participantRepo repository.ConversationParticipantRepository,
	messageRepo repository.ConversationMessageRepository,
) *ConversationService {
	return &ConversationService{
		convRepo:        convRepo,
		participantRepo: participantRepo,
		messageRepo:     messageRepo,
	}
}

// Conversation methods

func (s *ConversationService) CreateConversation(ctx context.Context, conv *models.Conversation, recipientIDs []uint) error {
	if conv.Subject == "" {
		return errors.New("conversation subject is required")
	}
	if conv.WorkflowState == "" {
		conv.WorkflowState = "active"
	}
	if conv.LastMessageAt.IsZero() {
		conv.LastMessageAt = time.Now()
	}

	if err := s.convRepo.Create(ctx, conv); err != nil {
		return err
	}

	// Add the creator as a participant
	creatorParticipant := &models.ConversationParticipant{
		ConversationID: conv.ID,
		UserID:         conv.CreatedByUserID,
		WorkflowState:  "active",
	}
	if err := s.participantRepo.Create(ctx, creatorParticipant); err != nil {
		return err
	}

	// Add all recipients as participants
	for _, recipientID := range recipientIDs {
		if recipientID == conv.CreatedByUserID {
			continue // skip if recipient is also the creator
		}
		participant := &models.ConversationParticipant{
			ConversationID: conv.ID,
			UserID:         recipientID,
			WorkflowState:  "active",
		}
		if err := s.participantRepo.Create(ctx, participant); err != nil {
			return err
		}
	}

	return nil
}

// GetConversation — 13.1.D: accountID threads from handler.
// Cross-tenant returns gorm.ErrRecordNotFound; the handler surfaces 404.
func (s *ConversationService) GetConversation(ctx context.Context, id, accountID uint) (*models.Conversation, error) {
	return s.convRepo.FindByID(ctx, id, accountID)
}

func (s *ConversationService) UpdateConversation(ctx context.Context, conv *models.Conversation) error {
	return s.convRepo.Update(ctx, conv)
}

func (s *ConversationService) ListByUser(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conversation], error) {
	return s.convRepo.ListByUserID(ctx, userID, accountID, params)
}

// Participant methods

func (s *ConversationService) AddParticipant(ctx context.Context, conversationID, userID uint) error {
	participant := &models.ConversationParticipant{
		ConversationID: conversationID,
		UserID:         userID,
		WorkflowState:  "active",
	}
	return s.participantRepo.Create(ctx, participant)
}

func (s *ConversationService) RemoveParticipant(ctx context.Context, conversationID, userID uint) error {
	return s.participantRepo.Delete(ctx, conversationID, userID)
}

func (s *ConversationService) GetParticipants(ctx context.Context, conversationID uint) ([]models.ConversationParticipant, error) {
	return s.participantRepo.ListByConversationID(ctx, conversationID)
}

// Message methods

func (s *ConversationService) CreateMessage(ctx context.Context, msg *models.ConversationMessage) error {
	if msg.Body == "" {
		return errors.New("message body is required")
	}
	if msg.WorkflowState == "" {
		msg.WorkflowState = "active"
	}

	if err := s.messageRepo.Create(ctx, msg); err != nil {
		return err
	}

	// Update conversation's LastMessageAt. Internal callback — pass
	// accountID=0 so the lookup isn't scoped (the caller has already
	// passed the participant gate, and the repo lookup here is a
	// best-effort metadata refresh, not a tenant-keyed read).
	conv, err := s.convRepo.FindByID(ctx, msg.ConversationID, 0)
	if err != nil {
		return err
	}
	conv.LastMessageAt = msg.CreatedAt
	return s.convRepo.Update(ctx, conv)
}

func (s *ConversationService) GetMessage(ctx context.Context, id uint) (*models.ConversationMessage, error) {
	return s.messageRepo.FindByID(ctx, id)
}

func (s *ConversationService) ListMessages(ctx context.Context, conversationID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ConversationMessage], error) {
	return s.messageRepo.ListByConversationID(ctx, conversationID, params)
}

func (s *ConversationService) DeleteMessage(ctx context.Context, id uint) error {
	return s.messageRepo.Delete(ctx, id)
}

// Mark as read

func (s *ConversationService) MarkConversationAsRead(ctx context.Context, conversationID, userID uint) error {
	participant, err := s.participantRepo.FindByConversationAndUser(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	now := time.Now()
	participant.LastReadAt = &now
	return s.participantRepo.Update(ctx, participant)
}
