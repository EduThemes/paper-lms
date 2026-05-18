package models

import "time"

type LearningOutcomeGroup struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType   string    `json:"context_type" gorm:"not null"`
	ContextID     uint      `json:"context_id" gorm:"not null;index"`
	ParentGroupID *uint     `json:"parent_outcome_group_id" gorm:"index"`
	Title         string    `json:"title" gorm:"not null"`
	Description   string    `json:"description" gorm:"type:text"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
