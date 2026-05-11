package models

import "time"

type QuizQuestion struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	QuizID              uint      `json:"quiz_id" gorm:"not null;index"`
	QuizQuestionGroupID *uint     `json:"quiz_question_group_id" gorm:"index"`
	Position            int       `json:"position"`
	QuestionType      string    `json:"question_type" gorm:"not null"` // multiple_choice, true_false, short_answer, essay, matching, fill_in_multiple_blanks, numerical_question
	QuestionText      string    `json:"question_text" gorm:"type:text;not null"`
	PointsPossible    *float64  `json:"points_possible"`
	Answers           string    `json:"answers" gorm:"type:jsonb"` // JSON array: [{id, text, comments, weight}]
	CorrectComments   string    `json:"correct_comments" gorm:"type:text"`
	IncorrectComments string    `json:"incorrect_comments" gorm:"type:text"`
	NeutralComments   string    `json:"neutral_comments" gorm:"type:text"`
	WorkflowState     string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	// Wave A2 additions — both nullable so existing rows are unaffected.
	BankItemID *uint `json:"bank_item_id" gorm:"index"`
	StimulusID *uint `json:"stimulus_id" gorm:"index"`
}
