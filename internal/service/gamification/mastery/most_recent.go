package mastery

// MostRecent uses only the latest attempt — the one-shot-certification
// model. Ties broken by slice order (later index wins) since OccurredAt
// equality means the upstream emitted the same instant.
type MostRecent struct{}

func (MostRecent) Method() Method { return MethodMostRecent }

func (MostRecent) Calculate(events []Event, _ Params) State {
	if len(events) == 0 {
		return State{}
	}
	latest := events[0]
	for _, e := range events[1:] {
		if !e.OccurredAt.Before(latest.OccurredAt) {
			latest = e
		}
	}
	v := clamp01(latest.Score)
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  latest.OccurredAt,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

