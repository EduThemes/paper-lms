package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conversationMessageRepo struct {
	db *gorm.DB
}

func NewConversationMessageRepository(db *gorm.DB) repository.ConversationMessageRepository {
	return &conversationMessageRepo{db: db}
}

func (r *conversationMessageRepo) Create(ctx context.Context, message *models.ConversationMessage) error {
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *conversationMessageRepo) FindByID(ctx context.Context, id uint) (*models.ConversationMessage, error) {
	var message models.ConversationMessage
	if err := r.db.WithContext(ctx).First(&message, id).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *conversationMessageRepo) Update(ctx context.Context, message *models.ConversationMessage) error {
	return r.db.WithContext(ctx).Save(message).Error
}

func (r *conversationMessageRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.ConversationMessage{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *conversationMessageRepo) ListByConversationID(ctx context.Context, conversationID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.ConversationMessage], error) {
	var messages []models.ConversationMessage
	var count int64

	query := r.db.WithContext(ctx).Model(&models.ConversationMessage{}).Where("conversation_id = ? AND workflow_state != ?", conversationID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at ASC").Find(&messages).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.ConversationMessage]{
		Items:      messages,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
