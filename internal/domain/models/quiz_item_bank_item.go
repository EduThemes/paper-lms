package models

import "time"

// QuizItemBankItem is a reusable question template stored inside a QuizItemBank.
// Its shape mirrors the gradable fields of QuizQuestion so an instructor can
// copy it into a quiz (creating a QuizQuestion whose BankItemID points back here).
type QuizItemBankItem struct {
	ID                uint      `json:"id" gorm:"column:id;primaryKey"`
	BankID            uint      `json:"bank_id" gorm:"not null;index"`
	Position          int       `json:"position"`
	QuestionType      string    `json:"question_type" gorm:"not null"`
	QuestionText      string    `json:"question_text" gorm:"type:text;not null"`
	PointsPossible    *float64  `json:"points_possible"`
	Answers           string    `json:"answers" gorm:"type:jsonb"`
	CorrectComments   string    `json:"correct_comments" gorm:"type:text"`
	IncorrectComments string    `json:"incorrect_comments" gorm:"type:text"`
	NeutralComments   string    `json:"neutral_comments" gorm:"type:text"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (QuizItemBankItem) TableName() string {
	return "quiz_item_bank_items"
}
