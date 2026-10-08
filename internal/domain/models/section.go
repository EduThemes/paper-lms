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
	// RestrictEnrollmentsToSectionDates makes StartAt/EndAt bound
	// participation for the section's enrollments (migration 000144).
	RestrictEnrollmentsToSectionDates bool `json:"restrict_enrollments_to_section_dates" gorm:"not null;default:false"`
	// NonxlistCourseID is the course a cross-listed section came from;
	// nil when the section is not cross-listed (migration 000144).
	NonxlistCourseID *uint     `json:"nonxlist_course_id" gorm:"index"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
