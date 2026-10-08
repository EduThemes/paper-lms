package models

import "time"

// QuestionBank is Paper's single question-bank system (Canvas
// AssessmentQuestionBank). It is owned by a course; with
// SharedWithAccount set, staff of every course in the same account can
// draw from it (question groups), pull or copy its questions, but only the
// owning course edits it. The former quiz_item_banks rows were copied in
// by migration 000104 and the /quiz_item_banks API now reads and writes
// these tables.
type QuestionBank struct {
	ID            uint      `json:"id" gorm:"column:id;primaryKey"`
	CourseID      uint      `json:"course_id" gorm:"index"`
	Title         string    `json:"title"`
	WorkflowState string    `json:"workflow_state" gorm:"default:'active'"` // active, deleted
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// Quiz-items lane (migration 000104).
	Description       string `json:"description" gorm:"type:text"`
	CreatedByUserID   *uint  `json:"created_by_user_id" gorm:"column:created_by_user_id"`
	AccountID         *uint  `json:"account_id" gorm:"column:account_id;index"`
	SharedWithAccount bool   `json:"shared_with_account" gorm:"column:shared_with_account;not null;default:false"`
	// LegacyQuizItemBankID records the quiz_item_banks row this bank was
	// copied from (unique when set; nullable so it stays *uint).
	LegacyQuizItemBankID *uint `json:"-" gorm:"column:legacy_quiz_item_bank_id"`
}

// QuestionBankEntry links a question to a question bank.
// QuestionData stores the question as JSON (same format as QuizQuestion).
type QuestionBankEntry struct {
	ID             uint      `json:"id" gorm:"column:id;primaryKey"`
	QuestionBankID uint      `json:"question_bank_id" gorm:"index"`
	QuestionName   string    `json:"question_name"`
	QuestionType   string    `json:"question_type"` // multiple_choice, true_false, short_answer, essay, numerical, matching, fill_in_multiple_blanks
	QuestionText   string    `json:"question_text"`
	PointsPossible float64   `json:"points_possible" gorm:"default:1"`
	Answers        string    `json:"answers" gorm:"type:text"` // JSON array of answer objects
	Feedback       string    `json:"feedback"`
	Position       int       `json:"position"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Quiz-items lane (migration 000104): per-outcome feedback carried over
	// from item-bank items, and the source item id of copied rows.
	CorrectComments          string `json:"correct_comments" gorm:"type:text"`
	IncorrectComments        string `json:"incorrect_comments" gorm:"type:text"`
	NeutralComments          string `json:"neutral_comments" gorm:"type:text"`
	LegacyQuizItemBankItemID *uint  `json:"-" gorm:"column:legacy_quiz_item_bank_item_id"`
}
