package models

import "time"

// SubmissionVersion is a snapshot of one submitted attempt, taken each
// time a student submits, so graders can page through earlier attempts
// (Canvas keeps these as submission versions / submission_history).
type SubmissionVersion struct {
	ID             uint       `json:"id" gorm:"column:id;primaryKey"`
	SubmissionID   uint       `json:"submission_id" gorm:"not null;uniqueIndex:idx_submission_versions_submission_attempt"`
	Attempt        int        `json:"attempt" gorm:"not null;uniqueIndex:idx_submission_versions_submission_attempt"`
	AssignmentID   uint       `json:"assignment_id" gorm:"not null;index"`
	UserID         uint       `json:"user_id" gorm:"not null;index"`
	SubmissionType *string    `json:"submission_type"`
	Body           *string    `json:"body" gorm:"type:text"`
	URL            *string    `json:"url" gorm:"column:url"`
	Attachments    *string    `json:"attachments" gorm:"type:text"`
	SubmittedAt    *time.Time `json:"submitted_at"`
	Late           bool       `json:"late" gorm:"not null;default:false"`
	CreatedAt      time.Time  `json:"created_at"`
}
