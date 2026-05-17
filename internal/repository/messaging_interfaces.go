package repository

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

type CalendarEventRepository interface {
	Create(ctx context.Context, event *models.CalendarEvent) error
	// FindByID — 13.1.D: context-polymorphic tenant scope.
	// User/Course/Group/Account context_type each filter through their tenant key.
	FindByID(ctx context.Context, id, accountID uint) (*models.CalendarEvent, error)
	Update(ctx context.Context, event *models.CalendarEvent) error
	Delete(ctx context.Context, id uint) error
	ListByContext(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.CalendarEvent], error)
	ListByContextAndDateRange(ctx context.Context, contextType string, contextID uint, startAt, endAt time.Time) ([]models.CalendarEvent, error)
}

type ConversationRepository interface {
	Create(ctx context.Context, conversation *models.Conversation) error
	FindByID(ctx context.Context, id uint) (*models.Conversation, error)
	Update(ctx context.Context, conversation *models.Conversation) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Conversation], error)
}

type ConversationParticipantRepository interface {
	Create(ctx context.Context, participant *models.ConversationParticipant) error
	FindByConversationAndUser(ctx context.Context, conversationID, userID uint) (*models.ConversationParticipant, error)
	Update(ctx context.Context, participant *models.ConversationParticipant) error
	Delete(ctx context.Context, conversationID, userID uint) error
	ListByConversationID(ctx context.Context, conversationID uint) ([]models.ConversationParticipant, error)
	ListByUserID(ctx context.Context, userID uint) ([]models.ConversationParticipant, error)
}

type ConversationMessageRepository interface {
	Create(ctx context.Context, message *models.ConversationMessage) error
	FindByID(ctx context.Context, id uint) (*models.ConversationMessage, error)
	Update(ctx context.Context, message *models.ConversationMessage) error
	Delete(ctx context.Context, id uint) error
	ListByConversationID(ctx context.Context, conversationID uint, params PaginationParams) (*PaginatedResult[models.ConversationMessage], error)
}

type NotificationPreferenceRepository interface {
	Create(ctx context.Context, prefs *models.NotificationPreference) error
	FindByUserID(ctx context.Context, userID uint) (*models.NotificationPreference, error)
	Update(ctx context.Context, prefs *models.NotificationPreference) error
	Delete(ctx context.Context, userID uint) error
}

type NotificationRepository interface {
	Create(ctx context.Context, notification *models.Notification) error
	FindByID(ctx context.Context, id uint) (*models.Notification, error)
	Update(ctx context.Context, notification *models.Notification) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Notification], error)
	MarkAsRead(ctx context.Context, userID, notificationID uint) error
	MarkAllAsRead(ctx context.Context, userID uint) error
	ListUnreadByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Notification], error)
}

type PlannerNoteRepository interface {
	Create(ctx context.Context, note *models.PlannerNote) error
	FindByID(ctx context.Context, id uint) (*models.PlannerNote, error)
	Update(ctx context.Context, note *models.PlannerNote) error
	Delete(ctx context.Context, id uint) error
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.PlannerNote], error)
}

type PlannerOverrideRepository interface {
	Create(ctx context.Context, override *models.PlannerOverride) error
	FindByID(ctx context.Context, id uint) (*models.PlannerOverride, error)
	Update(ctx context.Context, override *models.PlannerOverride) error
	Delete(ctx context.Context, id uint) error
	FindByUserAndPlannable(ctx context.Context, userID uint, plannableType string, plannableID uint) (*models.PlannerOverride, error)
	ListByUserID(ctx context.Context, userID uint) ([]models.PlannerOverride, error)
}
