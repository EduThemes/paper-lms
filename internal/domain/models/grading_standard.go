package models

import "time"

type GradingStandard struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	ContextType   string    `json:"context_type" gorm:"not null"`
	ContextID     uint      `json:"context_id" gorm:"not null"`
	Title         string    `json:"title" gorm:"not null"`
	Data          string    `json:"data" gorm:"type:jsonb;not null"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
