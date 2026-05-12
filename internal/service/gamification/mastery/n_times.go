package mastery

// NTimes requires N consecutive at-or-above-threshold scores to declare
// mastery — the repeated-skill-check model. Default N is 3 unless the
// rule overrides via Params.NTimesRequired. Stub implementation — full
// impl ships in the PR for Wave 1 task 7.
type NTimes struct{}

func (NTimes) Method() Method { return MethodNTimes }

func (NTimes) Calculate(events []Event, params Params) State {
	return State{}
}
