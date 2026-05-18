package models

import "time"

// BlueprintTemplate represents a blueprint course template that can sync content to associated courses.
type BlueprintTemplate struct {
	ID                     uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID               uint      `json:"course_id" gorm:"not null;index"`
	DefaultRestrictions    string    `json:"default_restrictions" gorm:"type:jsonb;default:'{}'"`
	UseDefaultRestrictions bool      `json:"use_default_restrictions" gorm:"default:true"`
	WorkflowState          string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
