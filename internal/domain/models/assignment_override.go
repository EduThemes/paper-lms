package models

import "time"

type AssignmentOverride struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	AssignmentID    uint       `json:"assignment_id" gorm:"not null;index"`
	Title           string     `json:"title"`
	DueAt           *time.Time `json:"due_at"`
	UnlockAt        *time.Time `json:"unlock_at"`
	LockAt          *time.Time `json:"lock_at"`
	AllDay          bool       `json:"all_day" gorm:"default:false"`
	AllDayDate      *time.Time `json:"all_day_date"`
	CourseSectionID *uint      `json:"course_section_id"`
	WorkflowState   string     `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
