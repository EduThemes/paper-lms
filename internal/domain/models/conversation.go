package models

import "time"

type Conversation struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	Subject         string    `json:"subject" gorm:"not null"`
	CreatedByUserID uint      `json:"created_by_user_id" gorm:"not null"`
	LastMessageAt   time.Time `json:"last_message_at" gorm:"index"`
	WorkflowState   string    `json:"workflow_state" gorm:"default:'active'"`
	// ContextType / ContextID tie a conversation to the course or group it
	// was sent in ("Course" / "Group"), like Canvas's context_code. Empty
	// / 0 for a context-free message. Drives the inbox course/group
	// filter (migration 000128).
	ContextType string    `json:"context_type" gorm:"column:context_type;type:text;index:idx_conversations_context,priority:1"`
	ContextID   uint      `json:"context_id" gorm:"column:context_id;index:idx_conversations_context,priority:2"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
