package models

import "time"

type QuizSubmissionAnswer struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	QuizSubmissionID uint      `json:"quiz_submission_id" gorm:"not null;index"`
	QuestionID       uint      `json:"question_id" gorm:"not null"`
	Answer           string    `json:"answer" gorm:"type:text"` // JSON: selected answer(s)
	Correct          *bool     `json:"correct"`
	Points           *float64  `json:"points"`
	// GradedVia records which grading pathway produced Points/Correct.
	// Added in Wave A1 (migration 000014). Values: "auto", "manual", or NULL
	// for legacy rows that pre-date the audit trail.
	GradedVia *string   `json:"graded_via" gorm:"size:32"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
