package models

import "time"

type Assignment struct {
	ID                 uint               `json:"id" gorm:"column:id;primaryKey"`
	CourseID           uint               `json:"course_id" gorm:"not null;index"`
	Name               string             `json:"name" gorm:"not null"`
	Description        string             `json:"description" gorm:"type:text"`
	DueAt              *time.Time         `json:"due_at"`
	UnlockAt           *time.Time         `json:"unlock_at"`
	LockAt             *time.Time         `json:"lock_at"`
	PointsPossible     *float64           `json:"points_possible"`
	GradingType        string             `json:"grading_type" gorm:"default:'points'"`                // points, percent, letter_grade, gpa_scale, pass_fail, not_graded
	SubmissionTypes    string             `json:"submission_types" gorm:"default:'online_text_entry'"` // comma-separated
	AssignmentGroupID  *uint              `json:"assignment_group_id" gorm:"index"`
	Position           int                `json:"position"`
	WorkflowState      AssignmentWorkflow `json:"workflow_state" gorm:"type:text;not null;default:'unpublished';index"`
	Published          bool               `json:"published" gorm:"default:false"`
	AnonymousGrading   bool               `json:"anonymous_grading" gorm:"default:false"`
	PostPolicy         string             `json:"post_policy" gorm:"default:'automatic'"` // automatic, manual
	PeerReviewsEnabled bool               `json:"peer_reviews_enabled" gorm:"default:false"`
	PeerReviewCount    int                `json:"peer_review_count" gorm:"default:0"`
	GroupCategoryID    *uint              `json:"group_category_id" gorm:"index"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}
