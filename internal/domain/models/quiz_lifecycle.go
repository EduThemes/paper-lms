package models

import (
	"time"

	"gorm.io/gorm"
)

// QuizExtension holds a student's moderation settings on one quiz (Canvas
// "Moderate this quiz": extra attempts, extra time, manual unlock;
// migration 000087). One row per (quiz, user).
type QuizExtension struct {
	ID     uint `json:"id" gorm:"column:id;primaryKey"`
	QuizID uint `json:"quiz_id" gorm:"not null;uniqueIndex:idx_quiz_extensions_quiz_user,priority:1"`
	UserID uint `json:"user_id" gorm:"not null;uniqueIndex:idx_quiz_extensions_quiz_user,priority:2;index:idx_quiz_extensions_user_id"`
	// ExtraAttempts are added to the quiz's allowed_attempts for this user.
	ExtraAttempts int `json:"extra_attempts" gorm:"not null;default:0"`
	// ExtraTime is extra minutes added to every timed attempt.
	ExtraTime int `json:"extra_time" gorm:"not null;default:0"`
	// ManuallyUnlocked lets the user start the quiz outside its
	// availability window (and keeps lock_at from capping the attempt).
	ManuallyUnlocked bool      `json:"manually_unlocked" gorm:"not null;default:false"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Quiz submission event types (Canvas quiz log auditing).
const (
	QuizEventSessionStarted     = "session_started"
	QuizEventQuestionAnswered   = "question_answered"
	QuizEventQuestionViewed     = "question_viewed"
	QuizEventQuestionFlagged    = "question_flagged"
	QuizEventPageBlurred        = "page_blurred"
	QuizEventPageFocused        = "page_focused"
	QuizEventSubmitted          = "submission_submitted"
	QuizEventAutoSubmitted      = "submission_auto_submitted"
	QuizEventAttemptExtended    = "attempt_extended"
	QuizEventManuallyGraded     = "manually_graded"
	QuizEventRegraded           = "regraded"
	QuizEventSessionResumed     = "session_resumed"
	QuizEventQuestionUnanswered = "question_unanswered"
)

// QuizSubmissionEvent is one entry in an attempt's audit log (migration
// 000088). Events come from the server (start, answer saves, submit,
// auto-submit, grading) and from the take page (focus/blur, views,
// flags). EventData is a small JSON object.
type QuizSubmissionEvent struct {
	ID               uint   `json:"id" gorm:"column:id;primaryKey"`
	QuizSubmissionID uint   `json:"quiz_submission_id" gorm:"not null;index:idx_quiz_submission_events_submission"`
	Attempt          int    `json:"attempt" gorm:"not null;default:1"`
	EventType        string `json:"event_type" gorm:"type:text;not null"`
	EventData        string `json:"event_data" gorm:"type:text"`
	// Source is "server" or "client"; client events are self-reported.
	Source          string     `json:"source" gorm:"type:text;not null;default:'server'"`
	ClientTimestamp *time.Time `json:"client_timestamp"`
	CreatedAt       time.Time  `json:"created_at"`
}

// BeforeCreate mirrors the SQL defaults for the NOT NULL columns GORM
// would otherwise write as zero values.
func (e *QuizSubmissionEvent) BeforeCreate(_ *gorm.DB) error {
	if e.Source == "" {
		e.Source = "server"
	}
	if e.Attempt == 0 {
		e.Attempt = 1
	}
	return nil
}

// Regrade options (Canvas quiz_question_regrades.regrade_option).
const (
	QuizRegradeCurrentCorrectOnly = "current_correct_only"
	QuizRegradeAwardBoth          = "current_and_previous_correct"
	QuizRegradeFullCredit         = "full_credit"
	QuizRegradeNoRegrade          = "no_regrade"
)

// QuizQuestionRegrade records one regrade run on a question after its
// answer key changed (migration 000089): the option chosen, the answer key it
// applied, who ran it and how many attempts' scores moved.
type QuizQuestionRegrade struct {
	ID                  uint      `json:"id" gorm:"column:id;primaryKey"`
	QuizID              uint      `json:"quiz_id" gorm:"not null;index:idx_quiz_question_regrades_quiz"`
	QuizQuestionID      uint      `json:"quiz_question_id" gorm:"not null"`
	RegradeOption       string    `json:"regrade_option" gorm:"type:text;not null"`
	AnswersSnapshot     string    `json:"answers_snapshot" gorm:"type:text"`
	RegradedByID        uint      `json:"regraded_by_id" gorm:"not null"`
	SubmissionsRegraded int       `json:"submissions_regraded" gorm:"not null;default:0"`
	SubmissionsChanged  int       `json:"submissions_changed" gorm:"not null;default:0"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
