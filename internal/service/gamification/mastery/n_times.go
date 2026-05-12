package mastery

import "sort"

// NTimes requires the last N events (default 3) to all clear a threshold
// (default 0.8). Models "demonstrate this skill three times in a row" —
// the repeated-skill-check pattern from D2L Release Conditions.
//
// Returns the mean of those last-N events as the State.Value. If the actor
// hasn't accumulated enough events, or any of the last-N fall below the
// threshold, returns a zero-value State (interpreted by callers as "not
// yet").
type NTimes struct{}

func (NTimes) Method() Method { return MethodNTimes }

func (NTimes) Calculate(events []Event, params Params) State {
	n := params.NTimesRequired
	if n <= 0 {
		n = 3
	}
	thr := params.NTimesThreshold
	if thr <= 0 {
		thr = 0.8
	}
	if len(events) < n {
		return State{}
	}

	// Defensive sort: callers don't have to pre-sort.
	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	last := sorted[len(sorted)-n:]
	var sum float64
	for _, e := range last {
		if e.Score < thr {
			return State{}
		}
		sum += e.Score
	}
	v := clamp01(sum / float64(n))
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  last[len(last)-1].OccurredAt,
	}
}
