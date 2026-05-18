package models

import "time"

type Course struct {
	ID                uint           `json:"id" gorm:"column:id;primaryKey"`
	AccountID         uint           `json:"account_id" gorm:"not null;default:1;index"`
	Name              string         `json:"name" gorm:"not null"`
	CourseCode        string         `json:"course_code" gorm:"not null"`
	SISCourseID       *string        `json:"sis_course_id" gorm:"column:sis_course_id;uniqueIndex"`
	WorkflowState     CourseWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'available';index"`
	EnrollmentTermID  *uint          `json:"enrollment_term_id" gorm:"index"`
	StartAt           *time.Time     `json:"start_at"`
	EndAt             *time.Time     `json:"end_at"`
	DefaultView       string         `json:"default_view" gorm:"default:'modules'"`
	UIMode            string         `json:"ui_mode" gorm:"column:ui_mode;default:'standard'"` // "standard", "k2", "3-5"
	SyllabusBody      string         `json:"syllabus_body" gorm:"type:text"`
	License           string         `json:"license" gorm:"default:'private'"`
	IsPublic          bool           `json:"is_public" gorm:"default:false"`
	ApplyGroupWeights bool           `json:"apply_assignment_group_weights" gorm:"default:false"`
	NavigationTabs    string         `json:"navigation_tabs" gorm:"type:text"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}
