package predicates

import (
	"context"
)

// SubmittedAssignment tests whether the actor has a submission for the
// given assignment, optionally constrained to a score range. The score
// range is expressed in points (not percent); leave both bounds nil for
// "any submission counts."
//
// Wave 1 wires this predicate end-to-end as the proof-of-pattern; the
// remaining six atomic predicates from PHASE6-WAVE1-PLAN.md task 6 follow.
type SubmittedAssignment struct {
	AssignmentID  uint     `json:"assignment_id"`
	MinScore      *float64 `json:"min_score,omitempty"`     // inclusive lower bound on the submission's score
	MaxScore      *float64 `json:"max_score,omitempty"`     // inclusive upper bound
	RequireOnTime bool     `json:"require_on_time,omitempty"` // if true, late submissions don't satisfy the predicate
}

func (p SubmittedAssignment) Kind() string { return "SubmittedAssignment" }

func (p SubmittedAssignment) Needs() Needs {
	return Needs{AssignmentIDs: []uint{p.AssignmentID}}
}

func (p SubmittedAssignment) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"assignment_id":   p.AssignmentID,
			"require_on_time": p.RequireOnTime,
		},
	}
	if p.MinScore != nil {
		trace.Params["min_score"] = *p.MinScore
	}
	if p.MaxScore != nil {
		trace.Params["max_score"] = *p.MaxScore
	}

	sub, ok := actor.Submissions[p.AssignmentID]
	if !ok || sub.SubmittedAt == nil {
		trace.Reason = "no submission for assignment"
		return false, trace
	}
	if p.RequireOnTime && !sub.OnTime {
		trace.Reason = "submission was late"
		return false, trace
	}
	if p.MinScore != nil {
		if sub.Score == nil {
			trace.Reason = "submission has no score yet"
			return false, trace
		}
		if *sub.Score < *p.MinScore {
			trace.Reason = "score below MinScore"
			return false, trace
		}
	}
	if p.MaxScore != nil {
		if sub.Score == nil {
			trace.Reason = "submission has no score yet"
			return false, trace
		}
		if *sub.Score > *p.MaxScore {
			trace.Reason = "score above MaxScore"
			return false, trace
		}
	}

	trace.Result = true
	return true, trace
}
