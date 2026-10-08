package models

import "time"

type AssignmentOverrideStudent struct {
	ID                   uint `json:"id" gorm:"column:id;primaryKey"`
	AssignmentOverrideID uint `json:"assignment_override_id" gorm:"not null;index"`
	// One ad-hoc override per student per assignment (Canvas); a student
	// may be in overrides on any number of assignments.
	UserID       uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_override_student_assignment_user,priority:2"`
	AssignmentID uint      `json:"assignment_id" gorm:"not null;index;uniqueIndex:idx_override_student_assignment_user,priority:1"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
