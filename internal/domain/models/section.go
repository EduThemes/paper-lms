package models

import "time"

type CourseSection struct {
	ID            uint       `json:"id" gorm:"column:id;primaryKey"`
	CourseID      uint       `json:"course_id" gorm:"not null;index"`
	Name          string     `json:"name" gorm:"not null"`
	SISSectionID  *string    `json:"sis_section_id" gorm:"column:sis_section_id;uniqueIndex"`
	WorkflowState string     `json:"workflow_state" gorm:"not null;default:'active'"`
	StartAt       *time.Time `json:"start_at"`
	EndAt         *time.Time `json:"end_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
