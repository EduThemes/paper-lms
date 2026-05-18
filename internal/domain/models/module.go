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
	WorkflowState             string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`

	Items []ContentTag `json:"items,omitempty" gorm:"foreignKey:ContextModuleID"`
}

func (ContextModule) TableName() string {
	return "context_modules"
}
