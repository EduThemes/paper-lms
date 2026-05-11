package models

import "time"

type AssignmentGroup struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CourseID      uint      `json:"course_id" gorm:"not null"`
	Name          string    `json:"name" gorm:"not null"`
	Position      int       `json:"position"`
	GroupWeight   float64   `json:"group_weight" gorm:"default:0"`
	Rules         string    `json:"rules" gorm:"type:jsonb;default:'{}'"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'available'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
