package models

import "time"

type QuizQuestionGroup struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	QuizID         uint      `json:"quiz_id" gorm:"not null;index"`
	Name           string    `json:"name"`
	PickCount      int       `json:"pick_count" gorm:"not null;default:1"` // how many questions to randomly pick
	PointsPerItem  *float64  `json:"points_per_item"`                      // override points for picked questions
	QuestionBankID *uint     `json:"question_bank_id"`                     // optional: pull from a question bank
	Position       int       `json:"position"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
