package models

import "time"

type Conversation struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	Subject         string    `json:"subject" gorm:"not null"`
	CreatedByUserID uint      `json:"created_by_user_id" gorm:"not null"`
	LastMessageAt   time.Time `json:"last_message_at" gorm:"index"`
	WorkflowState   string    `json:"workflow_state" gorm:"default:'active'"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
