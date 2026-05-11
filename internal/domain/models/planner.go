package models

import "time"

// PlannerNote - personal reminder notes created by students
type PlannerNote struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	UserID        uint      `json:"user_id" gorm:"not null;index"`
	Title         string    `json:"title" gorm:"type:varchar(255);not null"`
	Details       string    `json:"details" gorm:"type:text"`
	TodoDate      time.Time `json:"todo_date" gorm:"index"`
	CourseID      *uint     `json:"course_id,omitempty" gorm:"index"`
	WorkflowState string    `json:"workflow_state" gorm:"type:varchar(50);default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PlannerOverride - allows students to mark items as done or dismissed
type PlannerOverride struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	UserID         uint      `json:"user_id" gorm:"not null;index"`
	PlannableType  string    `json:"plannable_type" gorm:"type:varchar(50);not null;index"` // "assignment", "quiz", "discussion_topic", "wiki_page", "planner_note", "calendar_event", "announcement"
	PlannableID    uint      `json:"plannable_id" gorm:"not null;index"`
	MarkedComplete bool      `json:"marked_complete" gorm:"default:false"`
	Dismissed      bool      `json:"dismissed" gorm:"default:false"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
