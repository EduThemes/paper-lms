package models

import "time"

type ConversationMessage struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	ConversationID uint      `json:"conversation_id" gorm:"not null;index"`
	UserID         uint      `json:"user_id" gorm:"not null"`
	Body           string    `json:"body" gorm:"type:text;not null"`
	WorkflowState  string    `json:"workflow_state" gorm:"default:'active'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
