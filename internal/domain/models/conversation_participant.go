package models

import (
	"time"

	"gorm.io/gorm"
)

// Per-participant inbox states (Canvas's conversation_participants
// workflow_state). "active" is the pre-000102 value and reads as "read".
const (
	ConversationParticipantUnread   = "unread"
	ConversationParticipantRead     = "read"
	ConversationParticipantArchived = "archived"
	// ConversationParticipantRemoved marks a participant taken off the
	// conversation; such rows are excluded from every participant lookup.
	ConversationParticipantRemoved = "deleted"
)

type ConversationParticipant struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	ConversationID uint       `json:"conversation_id" gorm:"not null;uniqueIndex:idx_conv_user"`
	UserID         uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_conv_user;index:idx_conversation_participants_user_state,priority:1"`
	LastReadAt     *time.Time `json:"last_read_at"`
	WorkflowState  string     `json:"workflow_state" gorm:"default:'read';index:idx_conversation_participants_user_state,priority:2"`
	// Starred is the participant's own flag (Canvas "starred").
	Starred bool `json:"starred" gorm:"not null;default:false"`
	// ClearedAt is set when the participant deletes the conversation from
	// their inbox: messages up to this instant are hidden from them, and a
	// later message brings the conversation back (migration 000102).
	ClearedAt *time.Time `json:"cleared_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// BeforeCreate mirrors the SQL DEFAULT so a zero-value state is never
// written as ''.
func (p *ConversationParticipant) BeforeCreate(_ *gorm.DB) error {
	if p.WorkflowState == "" {
		p.WorkflowState = ConversationParticipantRead
	}
	return nil
}
