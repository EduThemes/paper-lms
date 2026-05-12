package mastery

// Highest uses the single best score across all attempts — the
// best-effort-credential model. Stub implementation — full impl ships in
// the PR for Wave 1 task 7.
type Highest struct{}

func (Highest) Method() Method { return MethodHighest }

func (Highest) Calculate(events []Event, params Params) State {
	return State{}
}
