package models

import "time"

type CoursePace struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	CourseID        uint       `json:"course_id" gorm:"not null;index"`
	UserID          *uint      `json:"user_id" gorm:"index"`           // student-specific pace
	CourseSectionID *uint      `json:"course_section_id" gorm:"index"` // section-specific pace
	WorkflowState   string     `json:"workflow_state" gorm:"not null;default:'unpublished'"` // unpublished, active, deleted
	EndDate         *time.Time `json:"end_date"`
	ExcludeWeekends bool       `json:"exclude_weekends" gorm:"default:true"`
	HardEndDates    bool       `json:"hard_end_dates" gorm:"default:false"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
