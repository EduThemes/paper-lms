package models

import "time"

// Late policy statuses an instructor can set on a submission (Canvas
// submissions.late_policy_status). Nil means the status is computed
// from the submission and due dates.
const (
	LatePolicyStatusNone     = "none"
	LatePolicyStatusMissing  = "missing"
	LatePolicyStatusLate     = "late"
	LatePolicyStatusExtended = "extended"
)

// ValidLatePolicyStatus reports whether s is a settable status.
func ValidLatePolicyStatus(s string) bool {
	switch s {
	case LatePolicyStatusNone, LatePolicyStatusMissing, LatePolicyStatusLate, LatePolicyStatusExtended:
		return true
	}
	return false
}

// SubmissionVersion is one attempt of a submission, kept when the
// student resubmits so earlier work is not lost (Canvas
// submission_versions / submission history). The grade fields follow
// the grading of that attempt.
type SubmissionVersion struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	SubmissionID   uint       `json:"submission_id" gorm:"not null;uniqueIndex:idx_submission_versions_submission_attempt,priority:1"`
	Attempt        int        `json:"attempt" gorm:"not null;uniqueIndex:idx_submission_versions_submission_attempt,priority:2"`
	AssignmentID   uint       `json:"assignment_id" gorm:"not null;index"`
	UserID         uint       `json:"user_id" gorm:"not null;index"`
	SubmissionType *string    `json:"submission_type"`
	Body           *string    `json:"body" gorm:"type:text"`
	URL            *string    `json:"url" gorm:"column:url"`
	Attachments    *string    `json:"attachments" gorm:"type:text"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	Late           bool       `json:"late" gorm:"default:false"`
	Score          *float64   `json:"score"`
	Grade          *string    `json:"grade"`
	EnteredScore   *float64   `json:"entered_score"`
	GraderID       *uint      `json:"grader_id"`
	GradedAt       *time.Time `json:"graded_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// CourseScoreOverride is an instructor's final grade override for one
// student (Canvas scores.override_score). GradingPeriodID 0 is the
// course-level (all periods) override.
type CourseScoreOverride struct {
	ID              uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID        uint      `json:"course_id" gorm:"not null;uniqueIndex:idx_course_score_overrides_scope,priority:1"`
	UserID          uint      `json:"user_id" gorm:"not null;index;uniqueIndex:idx_course_score_overrides_scope,priority:2"`
	GradingPeriodID uint      `json:"grading_period_id" gorm:"not null;default:0;uniqueIndex:idx_course_score_overrides_scope,priority:3"`
	OverrideScore   float64   `json:"override_score" gorm:"not null"`
	UpdatedByID     uint      `json:"updated_by_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
