// Package mastery implements the six interchangeable mastery-decay
// algorithms surfaced by the OutcomeMastery predicate's calc_method
// parameter.
//
// All six methods take the same (events, params) input and produce the
// same MasteryState output, so they can be swapped per-outcome or per-rule
// without affecting downstream consumers. Defaults per audience are
// documented in PHASE6-WAVE1-PLAN.md §"Decisions resolved":
//
//   - khan_spaced_retrieval — K-12 formative practice (math facts, vocab)
//   - decaying_average      — HigherEd rubric-graded summative
//   - most_recent           — one-shot certifications
//   - highest               — best-effort credentials
//   - n_times               — repeated skill checks (default n=3)
//   - weighted_average      — teacher-weighted multi-source
//
// Wave 1 ships these as zero-value stubs that satisfy the interface so the
// predicate evaluator wires up cleanly. Full implementations land in the
// PR for task 7 of the Wave 1 plan.
package mastery

import (
	"time"
)

// Method identifies one of the six calc methods.
type Method string

const (
	MethodKhanSpacedRetrieval Method = "khan_spaced_retrieval"
	MethodDecayingAverage     Method = "decaying_average"
	MethodMostRecent          Method = "most_recent"
	MethodHighest             Method = "highest"
	MethodNTimes              Method = "n_times"
	MethodWeightedAverage     Method = "weighted_average"
)

// Event is the minimal mastery-relevant fact a calculator consumes. The
// full event-store row carries more context; calculators only need the
// shape below.
type Event struct {
	OccurredAt time.Time
	Score      float64 // 0.0 – 1.0 (normalized)
	Weight     float64 // teacher-assigned weight, 1.0 default
}

// Params bundles the dial settings each calc method supports. Each method
// ignores fields it doesn't use.
type Params struct {
	// DecayingAverageRecentWeight: blend factor for decaying_average.
	// Default 0.65 (recent) / 0.35 (prior) per the plan.
	DecayingAverageRecentWeight float64
	// NTimesRequired: how many consecutive at-or-above-threshold scores
	// are needed for n_times. Default 3.
	NTimesRequired int
	// NTimesThreshold: the per-attempt threshold for n_times (e.g. 0.8).
	NTimesThreshold float64
	// HalfLifeDays: spaced-retrieval half-life for khan_spaced_retrieval.
	HalfLifeDays float64
}

// State is the calculator's output. Value is the mastery probability or
// score on a 0–1 scale; Level is the discretized bucket.
type State struct {
	Value float64
	Level string
	AsOf  time.Time
}

// Calculator is the common interface implemented by each of the six
// methods. Wave 1 stubs return zero values to keep callers honest about
// uninitialized output.
type Calculator interface {
	Method() Method
	Calculate(events []Event, params Params) State
}
