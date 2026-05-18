package models

import "time"

type CoursePace struct {
	ID              uint               `json:"id" gorm:"column:id;primaryKey"`
	CourseID        uint               `json:"course_id" gorm:"not null;index"`
	UserID          *uint              `json:"user_id" gorm:"index"`           // student-specific pace
	CourseSectionID *uint              `json:"course_section_id" gorm:"index"` // section-specific pace
	WorkflowState   CoursePaceWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'unpublished'"`
	EndDate         *time.Time         `json:"end_date"`
	ExcludeWeekends bool               `json:"exclude_weekends" gorm:"default:true"`
	HardEndDates    bool               `json:"hard_end_dates" gorm:"default:false"`
	PublishedAt     *time.Time         `json:"published_at"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}
