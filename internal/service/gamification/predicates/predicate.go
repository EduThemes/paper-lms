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

	// Needs declares the slice of ActorSnapshot state this predicate reads.
	// The snapshot loader unions the Needs across every predicate in a
	// rule's condition_set tree and issues one targeted query per slice
	// rather than full-user dumps. Composite predicates (ConditionSet)
	// return the union of their children's Needs.
	Needs() Needs
}

// Needs is the per-predicate declaration of which ActorSnapshot slices —
// and which specific IDs within those slices — the predicate touches at
// evaluation time. The snapshot loader builds a union of Needs across a
// rule's full condition_set tree, then issues one targeted query per
// non-empty field. Empty fields mean "this loader pass skips that slice."
//
// Adding a new predicate type that reads a new ActorSnapshot slice
// requires adding a corresponding field here so the loader knows to
// hydrate it. The factory (Sprint C task) is the canonical place to add
// a discriminator.
type Needs struct {
	AssignmentIDs   []uint   `json:"assignment_ids,omitempty"`
	QuizIDs         []uint   `json:"quiz_ids,omitempty"`
	ContentIDs      []uint   `json:"content_ids,omitempty"`
	OutcomeIDs      []uint   `json:"outcome_ids,omitempty"`
	BadgeIDs        []uint   `json:"badge_ids,omitempty"`
	CurrencyCodes   []string `json:"currency_codes,omitempty"`
	WantEnrollments bool     `json:"want_enrollments,omitempty"`
	WantLastLogin   bool     `json:"want_last_login,omitempty"`
}

// Union returns the deduplicated set-union of two Needs. The result's
// slices are sorted ascending for determinism so test assertions can
// compare needs without ordering noise.
func Union(a, b Needs) Needs {
	return Needs{
		AssignmentIDs:   mergeUints(a.AssignmentIDs, b.AssignmentIDs),
		QuizIDs:         mergeUints(a.QuizIDs, b.QuizIDs),
		ContentIDs:      mergeUints(a.ContentIDs, b.ContentIDs),
		OutcomeIDs:      mergeUints(a.OutcomeIDs, b.OutcomeIDs),
		BadgeIDs:        mergeUints(a.BadgeIDs, b.BadgeIDs),
		CurrencyCodes:   mergeStrings(a.CurrencyCodes, b.CurrencyCodes),
		WantEnrollments: a.WantEnrollments || b.WantEnrollments,
		WantLastLogin:   a.WantLastLogin || b.WantLastLogin,
	}
}

func mergeUints(a, b []uint) []uint {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(a)+len(b))
	out := make([]uint, 0, len(a)+len(b))
	for _, v := range a {
		if _, dup := seen[v]; !dup {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	for _, v := range b {
		if _, dup := seen[v]; !dup {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	sortUints(out)
	return out
}

func mergeStrings(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		if _, dup := seen[v]; !dup {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	for _, v := range b {
		if _, dup := seen[v]; !dup {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	sortStrings(out)
	return out
}

func sortUints(s []uint) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
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
	Submissions    map[uint]SubmissionState   // assignment_id → latest submission state
	QuizAttempts   map[uint]QuizState         // quiz_id → latest attempt
	ViewedContent  map[uint]ContentViewState  // content (object_id) → aggregated view stats
	OutcomeMastery map[uint]MasteryState      // outcome_id → calc'd mastery
	WalletBalances map[uint]int64             // currency_type_id → balance
	CurrencyByCode map[string]uint            // resolve "xp" → currency_type_id
	EarnedBadges   []uint
	Enrollments    []EnrollmentState
	LastLogin      time.Time
}

// ContentViewState mirrors the content_views aggregate row that the
// snapshot loader hydrates from `internal/repository/postgres/content_view.go`.
// Predicates compare against ViewCount and TotalSeconds for "watched X
// times" / "watched at least N seconds" gating.
type ContentViewState struct {
	ObjectID      uint
	ViewCount     int
	TotalSeconds  int64
	FirstViewedAt time.Time
	LastViewedAt  time.Time
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
