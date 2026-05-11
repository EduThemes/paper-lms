package models

import "time"

type QuestionBank struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CourseID      uint      `json:"course_id" gorm:"index"`
	Title         string    `json:"title"`
	WorkflowState string   `json:"workflow_state" gorm:"default:'active'"` // active, deleted
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// QuestionBankEntry links a question to a question bank.
// QuestionData stores the question as JSON (same format as QuizQuestion).
type QuestionBankEntry struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
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
}
