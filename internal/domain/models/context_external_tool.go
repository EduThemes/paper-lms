package models

import "time"

type ContextExternalTool struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType    string    `json:"context_type" gorm:"not null"` // Course, Account
	ContextID      uint      `json:"context_id" gorm:"not null;index"`
	DeveloperKeyID uint      `json:"developer_key_id" gorm:"not null;index"`
	Name           string    `json:"name" gorm:"not null"`
	Description    string    `json:"description"`
	URL            string    `json:"url" gorm:"column:url"`
	Domain         string    `json:"domain"`
	ConsumerKey    string    `json:"consumer_key"` // For LTI 1.1 backward compat
	SharedSecret   string    `json:"-"`            // For LTI 1.1 backward compat
	CustomFields   string    `json:"custom_fields" gorm:"type:text"`
	WorkflowState  string    `json:"workflow_state" gorm:"default:'active'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Associations
	DeveloperKey DeveloperKey `json:"-" gorm:"foreignKey:DeveloperKeyID"`
}
