package models

import "time"

type PeerReview struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	AssignmentID  uint      `json:"assignment_id" gorm:"index"`
	SubmissionID  uint      `json:"submission_id" gorm:"index"`
	ReviewerID    uint      `json:"reviewer_id" gorm:"index"`
	RevieweeID    uint      `json:"reviewee_id" gorm:"index"`
	WorkflowState string    `json:"workflow_state" gorm:"default:'assigned'"` // assigned, completed
	Score         *float64  `json:"score,omitempty"`
	Comments      string    `json:"comments"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
