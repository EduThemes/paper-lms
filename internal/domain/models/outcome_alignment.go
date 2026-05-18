package models

import "time"

// OutcomeAlignment links a learning outcome to an assignment for standards-based grading.
// When a submission for the aligned assignment is graded, a LearningOutcomeResult is
// automatically created/updated for the student.
type OutcomeAlignment struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	LearningOutcomeID uint      `json:"learning_outcome_id" gorm:"not null;uniqueIndex:idx_outcome_assignment"`
	AssignmentID      uint      `json:"assignment_id" gorm:"not null;uniqueIndex:idx_outcome_assignment;index"`
	CourseID          uint      `json:"course_id" gorm:"not null;index"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (OutcomeAlignment) TableName() string {
	return "outcome_alignments"
}
