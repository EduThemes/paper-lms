package models

import "time"

// BlueprintSubscription links a blueprint template to a child (associated) course.
type BlueprintSubscription struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	BlueprintTemplateID uint      `json:"blueprint_template_id" gorm:"not null;uniqueIndex:idx_template_child"`
	ChildCourseID       uint      `json:"child_course_id" gorm:"not null;uniqueIndex:idx_template_child"`
	WorkflowState       string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
