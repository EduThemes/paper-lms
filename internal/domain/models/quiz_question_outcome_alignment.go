package models

import "time"

// QuizQuestionOutcomeAlignment links a QuizQuestion to a LearningOutcome and
// records the mastery threshold (fraction of points required to count as
// "mastered"). Unique on (quiz_question_id, outcome_id).
//
// This is data-layer only: the quiz grader does not consume it yet.
type QuizQuestionOutcomeAlignment struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	QuizQuestionID   uint      `json:"quiz_question_id" gorm:"not null;uniqueIndex:idx_qq_outcome;index"`
	OutcomeID        uint      `json:"outcome_id" gorm:"not null;uniqueIndex:idx_qq_outcome;index"`
	MasteryThreshold float64   `json:"mastery_threshold" gorm:"not null;default:0.7"`
	CreatedAt        time.Time `json:"created_at"`
}

func (QuizQuestionOutcomeAlignment) TableName() string {
	return "quiz_question_outcome_alignments"
}
