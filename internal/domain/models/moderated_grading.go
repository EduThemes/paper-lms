package models

import "time"

// ModerationGrader records a grader taking part in a moderated
// assignment (Canvas moderation_graders). SlotTaken marks a provisional
// grader that counts toward the assignment's grader_count; the final
// grader takes part without occupying a slot. AnonymousID is the handle
// other graders see when graders are anonymous to each other.
type ModerationGrader struct {
	ID           uint      `json:"id" gorm:"column:id;primaryKey"`
	AssignmentID uint      `json:"assignment_id" gorm:"not null;uniqueIndex:idx_moderation_graders_assignment_user"`
	UserID       uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_moderation_graders_assignment_user;index"`
	AnonymousID  string    `json:"anonymous_id" gorm:"type:text;not null"`
	SlotTaken    bool      `json:"slot_taken" gorm:"not null;default:false"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProvisionalGrade is one grader's grade for a submission of a moderated
// assignment, held back from the student until the moderator publishes
// (Canvas moderated_grading_provisional_grades). Final marks the
// moderator's own custom grade.
type ProvisionalGrade struct {
	ID           uint       `json:"id" gorm:"column:id;primaryKey"`
	SubmissionID uint       `json:"submission_id" gorm:"not null;uniqueIndex:idx_provisional_grades_sub_scorer_final"`
	ScorerID     uint       `json:"scorer_id" gorm:"not null;uniqueIndex:idx_provisional_grades_sub_scorer_final;index"`
	Final        bool       `json:"final" gorm:"not null;default:false;uniqueIndex:idx_provisional_grades_sub_scorer_final"`
	Score        *float64   `json:"score"`
	Grade        *string    `json:"grade"`
	GradedAt     *time.Time `json:"graded_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName keeps Canvas's table name.
func (ProvisionalGrade) TableName() string { return "moderated_grading_provisional_grades" }

// ModerationSelection is the moderator's choice of which provisional
// grade becomes a student's grade when grades are published (Canvas
// moderated_grading_selections).
type ModerationSelection struct {
	ID                         uint      `json:"id" gorm:"column:id;primaryKey"`
	AssignmentID               uint      `json:"assignment_id" gorm:"not null;uniqueIndex:idx_moderation_selections_assignment_student"`
	StudentID                  uint      `json:"student_id" gorm:"not null;uniqueIndex:idx_moderation_selections_assignment_student"`
	SelectedProvisionalGradeID *uint     `json:"selected_provisional_grade_id"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// TableName keeps Canvas's table name.
func (ModerationSelection) TableName() string { return "moderated_grading_selections" }
