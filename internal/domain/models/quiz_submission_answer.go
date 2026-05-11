package models

import "time"

type QuizSubmissionAnswer struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	QuizSubmissionID uint      `json:"quiz_submission_id" gorm:"not null;index"`
	QuestionID       uint      `json:"question_id" gorm:"not null"`
	Answer           string    `json:"answer" gorm:"type:text"` // JSON: selected answer(s)
	Correct          *bool     `json:"correct"`
	Points           *float64  `json:"points"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
