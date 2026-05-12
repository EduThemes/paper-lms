package mastery

// WeightedAverage uses each event's teacher-assigned Weight in a standard
// weighted-mean: Σ(score·weight) / Σ(weight). Weight ≤ 0 defaults to 1.0
// so callers don't have to remember to set it for unweighted inputs.
// AsOf is the latest event's timestamp.
type WeightedAverage struct{}

func (WeightedAverage) Method() Method { return MethodWeightedAverage }

func (WeightedAverage) Calculate(events []Event, _ Params) State {
	if len(events) == 0 {
		return State{}
	}
	var num, den float64
	latest := events[0].OccurredAt
	for _, e := range events {
		w := e.Weight
		if w <= 0 {
			w = 1.0
		}
		num += e.Score * w
		den += w
		if e.OccurredAt.After(latest) {
			latest = e.OccurredAt
		}
	}
	if den == 0 {
		return State{AsOf: latest}
	}
	v := clamp01(num / den)
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  latest,
	}
}
