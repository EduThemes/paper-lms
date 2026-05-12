package mastery

import "sort"

// DecayingAverage blends the most recent attempt with the running average of
// priors at a default weight of 0.65 recent / 0.35 prior — Canvas's
// rubric-graded summative model. With only one event the result is just
// that event's score.
type DecayingAverage struct{}

func (DecayingAverage) Method() Method { return MethodDecayingAverage }

func (DecayingAverage) Calculate(events []Event, params Params) State {
	if len(events) == 0 {
		return State{}
	}
	w := params.DecayingAverageRecentWeight
	if w <= 0 {
		w = 0.65
	}
	if w > 1 {
		w = 1
	}

	sorted := make([]Event, len(events))
	copy(sorted, events)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	recent := sorted[len(sorted)-1]
	if len(sorted) == 1 {
		v := clamp01(recent.Score)
		return State{Value: v, Level: LevelFor(v), AsOf: recent.OccurredAt}
	}

	priors := sorted[:len(sorted)-1]
	var sum float64
	for _, e := range priors {
		sum += e.Score
	}
	priorAvg := sum / float64(len(priors))

	v := clamp01(recent.Score*w + priorAvg*(1-w))
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  recent.OccurredAt,
	}
}
