package models

import "time"

type Quiz struct {
	ID                 uint       `json:"id" gorm:"primaryKey"`
	CourseID           uint       `json:"course_id" gorm:"not null;index"`
	Title              string     `json:"title" gorm:"not null"`
	Description        string     `json:"description" gorm:"type:text"`
	QuizType           string     `json:"quiz_type" gorm:"default:'assignment'"` // practice_quiz, assignment, graded_survey, survey
	TimeLimit          *int       `json:"time_limit"`                             // minutes
	AllowedAttempts    int        `json:"allowed_attempts" gorm:"default:1"`      // -1 = unlimited
	DueAt              *time.Time `json:"due_at"`
	UnlockAt           *time.Time `json:"unlock_at"`
	LockAt             *time.Time `json:"lock_at"`
	PointsPossible     *float64   `json:"points_possible"`
	ShuffleAnswers     bool       `json:"shuffle_answers" gorm:"default:false"`
	ScoringPolicy      string     `json:"scoring_policy" gorm:"default:'keep_highest'"` // keep_highest, keep_latest, keep_average
	ShowCorrectAnswers bool       `json:"show_correct_answers" gorm:"default:true"`
	HideResults        string     `json:"hide_results"` // "", "always", "until_after_last_attempt"
	OneQuestionAtATime bool       `json:"one_question_at_a_time" gorm:"default:false"`
	CantGoBack         bool       `json:"cant_go_back" gorm:"default:false"`
	Published          bool       `json:"published" gorm:"default:false"`
	WorkflowState      string     `json:"workflow_state" gorm:"not null;default:'unpublished'"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
