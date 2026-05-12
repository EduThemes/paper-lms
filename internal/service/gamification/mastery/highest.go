package mastery

// Highest uses the single best score across all attempts — the
// best-effort-credential model. AsOf is the timestamp of whichever attempt
// produced the max; on ties the latest wins.
type Highest struct{}

func (Highest) Method() Method { return MethodHighest }

func (Highest) Calculate(events []Event, _ Params) State {
	if len(events) == 0 {
		return State{}
	}
	best := events[0]
	for _, e := range events[1:] {
		if e.Score > best.Score || (e.Score == best.Score && e.OccurredAt.After(best.OccurredAt)) {
			best = e
		}
	}
	v := clamp01(best.Score)
	return State{
		Value: v,
		Level: LevelFor(v),
		AsOf:  best.OccurredAt,
	}
}
