package models

import "time"

type Submission struct {
	ID             uint       `json:"id" gorm:"primaryKey"`
	AssignmentID   uint       `json:"assignment_id" gorm:"not null;uniqueIndex:idx_submission_assignment_user"`
	UserID         uint       `json:"user_id" gorm:"not null;uniqueIndex:idx_submission_assignment_user;index"`
	SubmissionType *string    `json:"submission_type"`
	Body           *string    `json:"body" gorm:"type:text"`
	URL            *string    `json:"url"`
	Score          *float64   `json:"score"`
	Grade          *string    `json:"grade"`
	GradedAt       *time.Time `json:"graded_at"`
	GraderID       *uint      `json:"grader_id"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	Attempt        int        `json:"attempt" gorm:"default:0"`
	Late           bool       `json:"late" gorm:"default:false"`
	Missing        bool       `json:"missing" gorm:"default:false"`
	Excused        bool       `json:"excused" gorm:"default:false"`
	Attachments    *string    `json:"attachments" gorm:"type:text"`
	WorkflowState  string     `json:"workflow_state" gorm:"not null;default:'unsubmitted';index"`
	PostedAt       *time.Time `json:"posted_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
