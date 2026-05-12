package mastery

import (
	"math"
	"sort"
)

// KhanSpacedRetrieval models Khan-style spaced-retrieval mastery:
//
//   - mastery decays exponentially over time at a half-life (default 7 days);
//   - a reattempt at-or-above the threshold (default 0.8) clamps the score
//     back to 1.0 and resets the decay anchor;
//   - a reattempt below the threshold replaces the current score with the
//     new score and resets the anchor.
//
// The decay formula is `score · 2^(-Δdays / halfLife)`, equivalently
// `score · exp(-ln(2)·Δdays / halfLife)`. This is the real half-life
// semantics (50% at one HalfLifeDays), matching Anki/SuperMemo conventions
// — the plan's `exp(-Δt/halfLife)` shorthand was a 1/e time-constant, not
// a half-life, despite the variable's name.
//
// The final value is the score after decaying from the last anchor to
// Params.Now (or to the latest event time if Now is zero). Anchor/score
// state is replayed across the event history in time order.
type KhanSpacedRetrieval struct{}

func (KhanSpacedRetrieval) Method() Method { return MethodKhanSpacedRetrieval }

func (KhanSpacedRetrieval) Calculate(events []Event, params Params) State {
	if len(events) == 0 {
		return State{}
	}
	halfLife := params.HalfLifeDays
	if halfLife <= 0 {
		halfLife = 7
	}
	thr := params.ReattemptThreshold
	if thr <= 0 {
		thr = 0.8
	}

	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	// Initialize at the first event — there's nothing to decay against yet.
	currentScore := sorted[0].Score
	if currentScore >= thr {
		currentScore = 1.0
	}
	anchor := sorted[0].OccurredAt

	for _, e := range sorted[1:] {
		deltaDays := e.OccurredAt.Sub(anchor).Hours() / 24.0
		decayed := currentScore * math.Exp(-math.Ln2*deltaDays/halfLife)

		if e.Score >= thr {
			// Reattempt-pass — Khan-style "back to mastered" on demonstrated
			// recall. The decayed prior is replaced wholesale.
			currentScore = 1.0
		} else {
			// Reattempt-below-threshold: blend the decayed prior with the
			// new (lower) signal by taking the more recent observation. The
			// plan specifies "replaces with the new score" — preserving that
			// keeps the spec exact and avoids the trap where an old high
			// score masks a recent regression. The `decayed` value is still
			// available in trace contexts if a future variant wants it.
			_ = decayed
			currentScore = e.Score
		}
		anchor = e.OccurredAt
	}

	// Final decay to "now."
	now := params.Now
	if now.IsZero() {
		now = sorted[len(sorted)-1].OccurredAt
	}
	deltaDays := now.Sub(anchor).Hours() / 24.0
	if deltaDays < 0 {
		// "Now" is before the last event — treat as zero elapsed.
		deltaDays = 0
	}
	final := currentScore * math.Exp(-math.Ln2*deltaDays/halfLife)
	v := clamp01(final)
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  now,
	}
}
