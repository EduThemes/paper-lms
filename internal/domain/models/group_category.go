package models

import "time"

type GroupCategory struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID      *uint     `json:"course_id" gorm:"index"`
	AccountID     *uint     `json:"account_id" gorm:"index"`
	Name          string    `json:"name" gorm:"not null"`
	SelfSignup    string    `json:"self_signup"` // enabled, restricted, or empty
	GroupLimit    *int      `json:"group_limit"`
	AutoLeader    string    `json:"auto_leader"` // first, random, or empty
	Role          string    `json:"role"`
	WorkflowState string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
