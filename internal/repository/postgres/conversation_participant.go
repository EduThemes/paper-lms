package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conversationParticipantRepo struct {
	db *gorm.DB
}

func NewConversationParticipantRepository(db *gorm.DB) repository.ConversationParticipantRepository {
	return &conversationParticipantRepo{db: db}
}

func (r *conversationParticipantRepo) Create(ctx context.Context, participant *models.ConversationParticipant) error {
	return r.db.WithContext(ctx).Create(participant).Error
}

func (r *conversationParticipantRepo) FindByConversationAndUser(ctx context.Context, conversationID, userID uint) (*models.ConversationParticipant, error) {
	var participant models.ConversationParticipant
	if err := r.db.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", conversationID, userID).First(&participant).Error; err != nil {
		return nil, err
	}
	return &participant, nil
}

func (r *conversationParticipantRepo) Update(ctx context.Context, participant *models.ConversationParticipant) error {
	return r.db.WithContext(ctx).Save(participant).Error
}

func (r *conversationParticipantRepo) Delete(ctx context.Context, conversationID, userID uint) error {
	return r.db.WithContext(ctx).Model(&models.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id = ?", conversationID, userID).
		Update("workflow_state", "deleted").Error
}

func (r *conversationParticipantRepo) ListByConversationID(ctx context.Context, conversationID uint) ([]models.ConversationParticipant, error) {
	var participants []models.ConversationParticipant
	if err := r.db.WithContext(ctx).Where("conversation_id = ? AND workflow_state != ?", conversationID, "deleted").Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}

func (r *conversationParticipantRepo) ListByUserID(ctx context.Context, userID uint) ([]models.ConversationParticipant, error) {
	var participants []models.ConversationParticipant
	if err := r.db.WithContext(ctx).Where("user_id = ? AND workflow_state != ?", userID, "deleted").Find(&participants).Error; err != nil {
		return nil, err
	}
	return participants, nil
}
