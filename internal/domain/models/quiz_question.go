package models

import "time"

type QuizQuestion struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	QuizID              uint      `json:"quiz_id" gorm:"not null;index"`
	QuizQuestionGroupID *uint     `json:"quiz_question_group_id" gorm:"index"`
	Position            int       `json:"position"`
	QuestionType        string    `json:"question_type" gorm:"not null"` // multiple_choice, true_false, short_answer, essay, matching, fill_in_multiple_blanks, numerical_question
	QuestionText        string    `json:"question_text" gorm:"type:text;not null"`
	PointsPossible      *float64  `json:"points_possible"`
	Answers             string    `json:"answers" gorm:"type:jsonb"` // JSON array: [{id, text, comments, weight}]
	CorrectComments     string    `json:"correct_comments" gorm:"type:text"`
	IncorrectComments   string    `json:"incorrect_comments" gorm:"type:text"`
	NeutralComments     string    `json:"neutral_comments" gorm:"type:text"`
	WorkflowState       string    `json:"workflow_state" gorm:"not null;default:'active'"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	// Wave A2 additions — both nullable so existing rows are unaffected.
	BankItemID *uint `json:"bank_item_id" gorm:"index"`
	StimulusID *uint `json:"stimulus_id" gorm:"index"`
	// QuestionBankEntryID is set on the per-quiz copy of a question-bank
	// entry drawn by a bank-linked question group (migration 000068). It
	// records provenance so later attempts reuse the same copy; these
	// copies are never treated as ungrouped quiz questions.
	QuestionBankEntryID *uint `json:"question_bank_entry_id" gorm:"column:question_bank_entry_id;index"`
}

// AuthoredQuizQuestions drops the per-attempt bank-draw copies (rows with
// QuestionBankEntryID set, materialized by a bank-linked question group).
// Those rows are only ever served through their group; anything that
// copies, exports, shares or diffs a quiz's questions must use the
// authored questions only, or every drawn entry becomes a fixed question
// in the copy.
func AuthoredQuizQuestions(qs []QuizQuestion) []QuizQuestion {
	out := make([]QuizQuestion, 0, len(qs))
	for _, q := range qs {
		if q.QuestionBankEntryID == nil {
			out = append(out, q)
		}
	}
	return out
}
