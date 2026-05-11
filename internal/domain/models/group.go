package models

import "time"

type Group struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	GroupCategoryID uint      `json:"group_category_id" gorm:"not null;index"`
	Name            string    `json:"name" gorm:"not null"`
	Description     string    `json:"description" gorm:"type:text"`
	MaxMembership   *int      `json:"max_membership"`
	IsPublic        bool      `json:"is_public" gorm:"default:false"`
	JoinLevel       string    `json:"join_level" gorm:"default:'invitation_only'"` // invitation_only, parent_context_auto_join, parent_context_request
	ContextType     string    `json:"context_type" gorm:"not null;default:'Course'"`
	ContextID       uint      `json:"context_id" gorm:"not null;index"`
	WorkflowState   string    `json:"workflow_state" gorm:"not null;default:'available'"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
