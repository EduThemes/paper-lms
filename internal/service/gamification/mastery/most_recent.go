package mastery

// MostRecent uses only the latest attempt — the one-shot-certification
// model. Stub implementation — full impl ships in the PR for Wave 1
// task 7.
type MostRecent struct{}

func (MostRecent) Method() Method { return MethodMostRecent }

func (MostRecent) Calculate(events []Event, params Params) State {
	return State{}
}
