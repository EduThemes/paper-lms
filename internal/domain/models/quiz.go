package models

import "time"

type Quiz struct {
	ID                 uint       `json:"id" gorm:"column:id;primaryKey"`
	CourseID           uint       `json:"course_id" gorm:"not null;index"`
	Title              string     `json:"title" gorm:"not null"`
	Description        string     `json:"description" gorm:"type:text"`
	QuizType           string     `json:"quiz_type" gorm:"default:'assignment'"` // practice_quiz, assignment, graded_survey, survey
	TimeLimit          *int       `json:"time_limit"`                            // minutes
	AllowedAttempts    int        `json:"allowed_attempts" gorm:"default:1"`     // -1 = unlimited
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

	// AssignmentID links a graded quiz (quiz_type assignment / graded_survey)
	// to its gradebook assignment (submission_types "online_quiz"), so quiz
	// scores flow through assignment groups, weights, late and post policy
	// (migration 000098). Practice quizzes and ungraded surveys have none.
	AssignmentID *uint `json:"assignment_id" gorm:"index:idx_quizzes_assignment_id"`
	// Answer-key release window (Canvas show_correct_answers_at /
	// hide_correct_answers_at) and "only after the last attempt".
	ShowCorrectAnswersAt          *time.Time `json:"show_correct_answers_at"`
	HideCorrectAnswersAt          *time.Time `json:"hide_correct_answers_at"`
	ShowCorrectAnswersLastAttempt bool       `json:"show_correct_answers_last_attempt" gorm:"not null;default:false"`
	// OneTimeResults: a student sees an attempt's results once, then they
	// are hidden (Canvas one_time_results).
	OneTimeResults bool `json:"one_time_results" gorm:"not null;default:false"`
}

// Quiz types (Canvas quiz_type).
const (
	QuizTypeAssignment   = "assignment"
	QuizTypePractice     = "practice_quiz"
	QuizTypeGradedSurvey = "graded_survey"
	QuizTypeSurvey       = "survey"
)

// Quiz scoring policies for multiple attempts (Canvas scoring_policy).
const (
	QuizScoringKeepHighest = "keep_highest"
	QuizScoringKeepLatest  = "keep_latest"
	QuizScoringKeepAverage = "keep_average"
)

// IsGraded reports whether the quiz counts toward the gradebook, i.e.
// whether it is backed by an online_quiz assignment.
func (q *Quiz) IsGraded() bool {
	if q == nil {
		return false
	}
	switch q.QuizType {
	case QuizTypeAssignment, QuizTypeGradedSurvey, "":
		return true
	}
	return false
}
