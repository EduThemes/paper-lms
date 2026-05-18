package models

import "time"

type ConversationParticipant struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	ConversationID uint       `json:"conversation_id" gorm:"not null;uniqueIndex:idx_conv_user"`
	UserID         uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_conv_user"`
	LastReadAt     *time.Time `json:"last_read_at"`
	WorkflowState  string     `json:"workflow_state" gorm:"default:'active'"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
