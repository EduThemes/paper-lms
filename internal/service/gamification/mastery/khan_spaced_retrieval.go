package mastery

// KhanSpacedRetrieval is the Khan-style mastery decay model: a learner's
// mastery probability decays over time and is bumped back up by
// at-or-above-threshold reattempts. Stub implementation — full impl ships
// in the PR for Wave 1 task 7.
type KhanSpacedRetrieval struct{}

func (KhanSpacedRetrieval) Method() Method { return MethodKhanSpacedRetrieval }

func (KhanSpacedRetrieval) Calculate(events []Event, params Params) State {
	return State{}
}
