package predicates

import (
	"context"
)

// SubmittedQuiz tests whether the actor has a quiz submission for the given
// quiz, optionally constrained to a score range. Mirrors SubmittedAssignment
// against QuizAttempts. Score bounds are in points (not percent); leave both
// nil for "any submission counts."
//
// QuizState in Wave 1 does not carry an OnTime flag (Paper LMS's quiz domain
// has no notion of lateness yet), so RequireOnTime is omitted here — add it
// once the model gains the field.
type SubmittedQuiz struct {
	QuizID   uint     `json:"quiz_id"`
	MinScore *float64 `json:"min_score,omitempty"` // inclusive lower bound on the submission's score
	MaxScore *float64 `json:"max_score,omitempty"` // inclusive upper bound
}

func (p SubmittedQuiz) Kind() string { return "SubmittedQuiz" }

func (p SubmittedQuiz) Needs() Needs {
	return Needs{QuizIDs: []uint{p.QuizID}}
}

func (p SubmittedQuiz) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"quiz_id": p.QuizID,
		},
	}
	if p.MinScore != nil {
		trace.Params["min_score"] = *p.MinScore
	}
	if p.MaxScore != nil {
		trace.Params["max_score"] = *p.MaxScore
	}

	attempt, ok := actor.QuizAttempts[p.QuizID]
	if !ok || attempt.SubmittedAt == nil {
		trace.Reason = "no submission for quiz"
		return false, trace
	}
	if p.MinScore != nil {
		if attempt.Score == nil {
			trace.Reason = "submission has no score yet"
			return false, trace
		}
		if *attempt.Score < *p.MinScore {
			trace.Reason = "score below MinScore"
			return false, trace
		}
	}
	if p.MaxScore != nil {
		if attempt.Score == nil {
			trace.Reason = "submission has no score yet"
			return false, trace
		}
		if *attempt.Score > *p.MaxScore {
			trace.Reason = "score above MaxScore"
			return false, trace
		}
	}

	trace.Result = true
	return true, trace
}
