package predicates

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
)

// OutcomeMastery tests whether the actor's cached mastery for an outcome is
// at or above MinLevel (one of novice|familiar|proficient|mastered).
//
// CalcMethod is an authoring hint that the snapshot loader honors when
// recomputing on the fly (Sprint C). Wave 1 trusts the cached MasteryState
// in the snapshot — if a rule sets CalcMethod, it has no effect on the
// predicate itself; the loader is responsible for materializing the snapshot
// using that method.
type OutcomeMastery struct {
	OutcomeID  uint   `json:"outcome_id"`
	MinLevel   string `json:"min_level"`             // novice | familiar | proficient | mastered
	CalcMethod string `json:"calc_method,omitempty"` // optional override hint; used by the snapshot loader (Sprint C)
}

func (p OutcomeMastery) Kind() string { return "OutcomeMastery" }

func (p OutcomeMastery) Needs() Needs {
	return Needs{OutcomeIDs: []uint{p.OutcomeID}}
}

func (p OutcomeMastery) Evaluate(_ context.Context, actor ActorSnapshot) (bool, Trace) {
	trace := Trace{
		Kind: p.Kind(),
		Params: map[string]any{
			"outcome_id": p.OutcomeID,
			"min_level":  p.MinLevel,
		},
	}
	if p.CalcMethod != "" {
		trace.Params["calc_method"] = p.CalcMethod
	}

	want := mastery.LevelOrdinal(p.MinLevel)
	if want < 0 {
		trace.Reason = "invalid MinLevel"
		return false, trace
	}

	state, ok := actor.OutcomeMastery[p.OutcomeID]
	if !ok {
		trace.Reason = "no cached mastery for outcome"
		return false, trace
	}

	have := mastery.LevelOrdinal(state.Level)
	if have < 0 {
		trace.Reason = "cached mastery level is not recognized"
		return false, trace
	}
	if have < want {
		trace.Reason = "mastery below MinLevel"
		return false, trace
	}

	trace.Result = true
	return true, trace
}
