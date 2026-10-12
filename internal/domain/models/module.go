package models

import "time"

type ContextModule struct {
	ID                        uint       `json:"id" gorm:"column:id;primaryKey"`
	CourseID                  uint       `json:"course_id" gorm:"not null;index"`
	Name                      string     `json:"name" gorm:"not null"`
	Position                  int        `json:"position"`
	UnlockAt                  *time.Time `json:"unlock_at"`
	EndAt                     *time.Time `json:"end_at"`
	RequireSequentialProgress bool       `json:"require_sequential_progress" gorm:"default:false"`
	// RequirementCount: 0 = complete every requirement, 1 = complete any
	// one (Canvas requirement_count). Migration 000092.
	RequirementCount int `json:"requirement_count" gorm:"not null;default:0"`
	WorkflowState             string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	Items []ContentTag `json:"items,omitempty" gorm:"foreignKey:ContextModuleID"`
}

func (ContextModule) TableName() string {
	return "context_modules"
}
