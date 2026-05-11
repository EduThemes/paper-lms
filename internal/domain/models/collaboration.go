package models

import "time"

type Collaboration struct {
	ID                uint      `json:"id" gorm:"primaryKey"`
	ContextType       string    `json:"context_type" gorm:"not null;index:idx_collab_context"`  // Course, Group
	ContextID         uint      `json:"context_id" gorm:"not null;index:idx_collab_context"`
	CollaborationType string    `json:"collaboration_type" gorm:"not null"` // google_docs, etherpad
	Title             string    `json:"title" gorm:"not null"`
	Description       string    `json:"description" gorm:"type:text"`
	URL               string    `json:"url"`
	DocumentID        string    `json:"document_id"`
	UserID            uint      `json:"user_id" gorm:"not null"`
	WorkflowState     string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
