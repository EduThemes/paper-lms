package mastery

// WeightedAverage uses each event's teacher-assigned Weight in a standard
// weighted-mean calculation — the explicit multi-source-mastery model.
// Stub implementation — full impl ships in the PR for Wave 1 task 7.
type WeightedAverage struct{}

func (WeightedAverage) Method() Method { return MethodWeightedAverage }

func (WeightedAverage) Calculate(events []Event, params Params) State {
	return State{}
}
