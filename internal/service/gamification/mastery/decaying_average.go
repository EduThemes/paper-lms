package mastery

// DecayingAverage blends the most recent attempt with the running average
// of priors at a default ratio of 0.65 / 0.35 (Canvas-style). Stub
// implementation — full impl ships in the PR for Wave 1 task 7.
type DecayingAverage struct{}

func (DecayingAverage) Method() Method { return MethodDecayingAverage }

func (DecayingAverage) Calculate(events []Event, params Params) State {
	return State{}
}
