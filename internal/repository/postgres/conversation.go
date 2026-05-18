package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type conversationRepo struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) repository.ConversationRepository {
	return &conversationRepo{db: db}
}

// conversationTenantFilter scopes conversations through the creator's
// account_id. The model has no direct account_id column; the
// CreatedByUserID FK joins to users.account_id. The handler-side
// participant gate (requireParticipant → 404) is the primary
// existence-leak boundary; this filter is the defense-in-depth layer
// the 13.1.D contract requires on every tenant-keyed read.
// accountID==0 disables (internal callers / background jobs).
const conversationTenantFilter = `created_by_user_id IN (SELECT id FROM users WHERE account_id = ?)`

func (r *conversationRepo) Create(ctx context.Context, conversation *models.Conversation) error {
	return r.db.WithContext(ctx).Create(conversation).Error
}

func (r *conversationRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Conversation, error) {
	var conversation models.Conversation
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		q = q.Where(conversationTenantFilter, accountID)
	}
	if err := q.First(&conversation, id).Error; err != nil {
		return nil, err
	}
	return &conversation, nil
}

func (r *conversationRepo) Update(ctx context.Context, conversation *models.Conversation) error {
	return r.db.WithContext(ctx).Save(conversation).Error
}

func (r *conversationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Conversation{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *conversationRepo) ListByUserID(ctx context.Context, userID, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Conversation], error) {
	var conversations []models.Conversation
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Conversation{}).
		Joins("JOIN conversation_participants ON conversation_participants.conversation_id = conversations.id").
		Where("conversation_participants.user_id = ? AND conversation_participants.workflow_state != ?", userID, "deleted").
		Where("conversations.workflow_state != ?", "deleted")

	if accountID != 0 {
		// Tenant filter must reference the qualified column when joining.
		query = query.Where("conversations.created_by_user_id IN (SELECT id FROM users WHERE account_id = ?)", accountID)
	}

	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("conversations.last_message_at DESC").Find(&conversations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Conversation]{
		Items:      conversations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
