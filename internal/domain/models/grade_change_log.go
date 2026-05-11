package models

import "time"

// GradeChangeLog is a specialized audit log for grade changes.
// Separate model for fast queries and compliance reporting.
type GradeChangeLog struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CourseID      uint      `json:"course_id" gorm:"index"`
	AssignmentID  uint      `json:"assignment_id" gorm:"index"`
	StudentID     uint      `json:"student_id" gorm:"index"`
	GraderID      uint      `json:"grader_id" gorm:"index"` // who made the change
	SubmissionID  uint      `json:"submission_id"`
	OldGrade      string    `json:"old_grade"`  // letter or points as string
	NewGrade      string    `json:"new_grade"`
	OldScore      *float64  `json:"old_score"`
	NewScore      *float64  `json:"new_score"`
	Excused       bool      `json:"excused"`
	GradingMethod string    `json:"grading_method"` // manual, auto_grade, rubric, speedgrader
	CreatedAt     time.Time `json:"created_at" gorm:"index"`
}
