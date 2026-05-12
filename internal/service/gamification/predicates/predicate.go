// Package predicates implements the unified predicate vocabulary that
// drives the Phase 6 gamification rules engine.
//
// A Predicate is a pure function over an ActorSnapshot — a frozen view of
// one user's state at one moment. This means:
//
//   - rules can be replayed at backfill time (new rule fires once over
//     each user's prior history without re-emitting events);
//   - predicates test cleanly without a database;
//   - many rules listening to the same event can share one snapshot load.
//
// The 24-predicate vocabulary is laid out in SYNTHESIS.md §1. Wave 1 only
// ships SubmittedAssignment + ConditionSet end-to-end; the remaining six
// from PHASE6-WAVE1-PLAN.md task 6 follow in a later PR.
package predicates

import (
	"context"
	"time"
)

// Predicate is the unit of evaluation. Implementations are pure: given the
// same ActorSnapshot they must return the same (result, trace).
type Predicate interface {
	// Kind identifies the predicate's JSONB discriminator (e.g.
	// "SubmittedAssignment", "ConditionSet"). Used by the trace writer and
	// by the JSONB → predicate factory.
	Kind() string

	// Evaluate returns the predicate's truth value plus a structured trace
	// for debuggability. The trace is stored on the rule_evaluation row's
	// predicate_state JSONB so teachers can ask "why didn't this fire?"
	// weeks later.
	Evaluate(ctx context.Context, actor ActorSnapshot) (bool, Trace)
}

// ActorSnapshot is the frozen view of one user passed to every predicate
// in a single rule evaluation. Loaders populate only the slices each
// predicate declares it needs — the rest stay nil.
//
// Most maps are keyed by the upstream object id (assignment id, quiz id,
// outcome id) as a uint, matching the rest of the Paper LMS schema.
type ActorSnapshot struct {
	UserID         uint
	TenantID       uint
	Now            time.Time
	Submissions    map[uint]SubmissionState // assignment_id → latest submission state
	QuizAttempts   map[uint]QuizState       // quiz_id → latest attempt
	OutcomeMastery map[uint]MasteryState    // outcome_id → calc'd mastery
	WalletBalances map[uint]int64           // currency_type_id → balance
	CurrencyByCode map[string]uint          // resolve "xp" → currency_type_id
	EarnedBadges   []uint
	Enrollments    []EnrollmentState
	LastLogin      time.Time
}

// SubmissionState captures the assignment-submission-level facts the
// predicate vocabulary cares about. Mirrors the relevant Paper LMS
// submission fields without forcing predicates to touch the model.
type SubmissionState struct {
	AssignmentID uint
	SubmittedAt  *time.Time
	Score        *float64
	PointsPossible float64
	WorkflowState string // 'submitted' | 'graded' | 'pending_review' | ...
	OnTime       bool
	AttemptCount int
}

// QuizState mirrors SubmissionState for quizzes.
type QuizState struct {
	QuizID       uint
	SubmittedAt  *time.Time
	Score        *float64
	PointsPossible float64
	WorkflowState string
	AttemptCount int
}

// MasteryState is the output of one of the six mastery calc methods.
// Level mirrors Khan-style buckets; Value carries the underlying numeric
// for predicates that compare on a continuous scale.
type MasteryState struct {
	OutcomeID  uint
	Value      float64   // 0.0–1.0, calc-method specific
	Level      string    // 'novice'|'familiar'|'proficient'|'mastered'
	CalcMethod string    // 'khan_spaced_retrieval' | 'decaying_average' | …
	AsOf       time.Time
}

// EnrollmentState records role-and-section context for enrollment-based
// predicates (EnrolledIn, DaysSinceEnrollment).
type EnrollmentState struct {
	CourseID    uint
	SectionID   *uint
	Role        string
	EnrolledAt  time.Time
}

// Trace is the structured record of one predicate evaluation. Nested for
// composite predicates so a ConditionSet's trace lists each child's
// outcome.
type Trace struct {
	Kind     string            `json:"kind"`
	Result   bool              `json:"result"`
	Reason   string            `json:"reason,omitempty"`
	Children []Trace           `json:"children,omitempty"`
	Params   map[string]any    `json:"params,omitempty"`
}
