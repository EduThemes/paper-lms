package gamification

import (
	"context"
	"fmt"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/mastery"
	"github.com/EduThemes/paper-lms/internal/service/gamification/predicates"
)

// DefaultContentObjectType is the polymorphic object_type the snapshot
// loader filters content_views on when the caller doesn't supply one.
// "Page" mirrors the primary ViewedContent use-case in Wave 1: gating a
// rule on whether the student has visited a lesson page.
const DefaultContentObjectType = "Page"

// SnapshotDeps bundles the repository dependencies the snapshot loader
// needs. Each one is consulted lazily — a Needs value with empty slices
// for a given facet skips that repo entirely so a rule with only a
// CurrencyThreshold predicate doesn't pay for submission/quiz/outcome
// lookups.
type SnapshotDeps struct {
	Submissions     repository.SubmissionRepository
	QuizSubmissions repository.QuizSubmissionRepository
	OutcomeResults  repository.LearningOutcomeResultRepository
	ContentViews    repository.ContentViewRepository
	Wallet          repository.GamificationWalletRepository
	CurrencyType    repository.GamificationCurrencyTypeRepository
}

// LoadSnapshot hydrates an ActorSnapshot for one user against the union
// of state slices declared in `needs`. Empty fields in Needs mean "skip
// that slice entirely" — the loader runs zero queries for unneeded
// slices so a rule with only a CurrencyThreshold predicate doesn't pay
// for submission/quiz/outcome lookups.
//
// The contentObjectType arg defaults to DefaultContentObjectType
// ("Page") if empty — the ViewedContent predicate's primary use case
// (lesson-progression gating). Future predicates can pass other object
// types ("ModuleItem", "WikiPage") once they need them.
//
// Wallet hydration is triggered by a non-empty `needs.CurrencyCodes`
// (the CurrencyThreshold predicate's input). CurrencyByCode and
// WalletBalances are populated together because CurrencyThreshold needs
// both: codes resolve to currency_type_ids, and balances key on those
// ids.
//
// Wave 1 does not populate Enrollments or LastLogin: the corresponding
// repositories don't yet exist. When `needs.WantEnrollments` or
// `needs.WantLastLogin` is true the loader silently leaves those fields
// at their zero value — predicates that depend on them will fail-with-
// reason rather than crash. Sprint D wires in the real loaders.
func LoadSnapshot(
	ctx context.Context,
	deps SnapshotDeps,
	userID, tenantID uint,
	needs predicates.Needs,
	contentObjectType string,
) (predicates.ActorSnapshot, error) {
	snap := predicates.ActorSnapshot{
		UserID:   userID,
		TenantID: tenantID,
		Now:      time.Now(),
	}

	if contentObjectType == "" {
		contentObjectType = DefaultContentObjectType
	}

	// Assignments → SubmissionState map keyed by AssignmentID.
	if len(needs.AssignmentIDs) > 0 {
		if deps.Submissions == nil {
			return snap, fmt.Errorf("LoadSnapshot: assignment_ids requested but Submissions repo is nil")
		}
		subs, err := deps.Submissions.ListByUserAndAssignmentIDs(ctx, userID, needs.AssignmentIDs)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list submissions: %w", err)
		}
		if len(subs) > 0 {
			snap.Submissions = make(map[uint]predicates.SubmissionState, len(subs))
			for _, s := range subs {
				snap.Submissions[s.AssignmentID] = predicates.SubmissionState{
					AssignmentID: s.AssignmentID,
					SubmittedAt:  s.SubmittedAt,
					Score:        s.Score,
					// PointsPossible isn't on the Submission model — the
					// predicate only uses Score bounds in Wave 1. Leave it
					// zero until Sprint D wires in the assignment lookup.
					PointsPossible: 0,
					WorkflowState:  s.WorkflowState,
					OnTime:         !s.Late,
					AttemptCount:   s.Attempt,
				}
			}
		}
	}

	// Quizzes → QuizState map keyed by QuizID.
	if len(needs.QuizIDs) > 0 {
		if deps.QuizSubmissions == nil {
			return snap, fmt.Errorf("LoadSnapshot: quiz_ids requested but QuizSubmissions repo is nil")
		}
		qsubs, err := deps.QuizSubmissions.ListByUserAndQuizIDs(ctx, userID, needs.QuizIDs)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list quiz submissions: %w", err)
		}
		if len(qsubs) > 0 {
			snap.QuizAttempts = make(map[uint]predicates.QuizState, len(qsubs))
			for _, q := range qsubs {
				snap.QuizAttempts[q.QuizID] = predicates.QuizState{
					QuizID:         q.QuizID,
					SubmittedAt:    q.FinishedAt,
					Score:          q.Score,
					PointsPossible: 0,
					WorkflowState:  q.WorkflowState,
					AttemptCount:   q.Attempt,
				}
			}
		}
	}

	// ContentViews → ContentViewState map keyed by ObjectID.
	if len(needs.ContentIDs) > 0 {
		if deps.ContentViews == nil {
			return snap, fmt.Errorf("LoadSnapshot: content_ids requested but ContentViews repo is nil")
		}
		views, err := deps.ContentViews.ListByUserAndObjectIDs(ctx, userID, contentObjectType, needs.ContentIDs)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list content views: %w", err)
		}
		if len(views) > 0 {
			snap.ViewedContent = make(map[uint]predicates.ContentViewState, len(views))
			for _, v := range views {
				snap.ViewedContent[v.ObjectID] = predicates.ContentViewState{
					ObjectID:      v.ObjectID,
					ViewCount:     v.ViewCount,
					TotalSeconds:  v.TotalSeconds,
					FirstViewedAt: v.FirstViewedAt,
					LastViewedAt:  v.LastViewedAt,
				}
			}
		}
	}

	// OutcomeResults → MasteryState keyed by LearningOutcomeID. Wave 1
	// stores the latest result per outcome as a degenerate mastery row
	// (Value = Percent if set, otherwise Score/Possible; Level via
	// mastery.LevelFor). Real per-rule calc_method selection lands in
	// Sprint D when the rule references it.
	if len(needs.OutcomeIDs) > 0 {
		if deps.OutcomeResults == nil {
			return snap, fmt.Errorf("LoadSnapshot: outcome_ids requested but OutcomeResults repo is nil")
		}
		results, err := deps.OutcomeResults.ListByUserAndOutcomeIDs(ctx, userID, needs.OutcomeIDs)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list outcome results: %w", err)
		}
		if len(results) > 0 {
			latest := make(map[uint]models.LearningOutcomeResult, len(results))
			for _, r := range results {
				prev, seen := latest[r.LearningOutcomeID]
				if !seen || resultIsLater(r, prev) {
					latest[r.LearningOutcomeID] = r
				}
			}
			snap.OutcomeMastery = make(map[uint]predicates.MasteryState, len(latest))
			for outcomeID, r := range latest {
				value := outcomeResultValue(r)
				asOf := outcomeResultTime(r)
				snap.OutcomeMastery[outcomeID] = predicates.MasteryState{
					OutcomeID:  outcomeID,
					Value:      value,
					Level:      mastery.LevelFor(value),
					CalcMethod: "", // per-rule selection happens in Sprint D
					AsOf:       asOf,
				}
			}
		}
	}

	// Wallet hydration: any currency need (CurrencyThreshold today, future
	// wallet-bound predicates tomorrow) triggers a single ListByTenant +
	// ListBalancesForUser pair. CurrencyByCode is built for every
	// tenant-defined currency so a predicate that resolves "xp" → id
	// succeeds even if the user's balance row hasn't been touched yet.
	// If a needed code has no tenant row, it's simply absent from the
	// map; CurrencyThreshold returns false-with-reason in that case.
	if len(needs.CurrencyCodes) > 0 {
		if deps.CurrencyType == nil {
			return snap, fmt.Errorf("LoadSnapshot: currency_codes requested but CurrencyType repo is nil")
		}
		if deps.Wallet == nil {
			return snap, fmt.Errorf("LoadSnapshot: currency_codes requested but Wallet repo is nil")
		}
		currencies, err := deps.CurrencyType.ListByTenant(ctx, tenantID)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list currencies: %w", err)
		}
		if len(currencies) > 0 {
			snap.CurrencyByCode = make(map[string]uint, len(currencies))
			for _, c := range currencies {
				// Last-write wins on duplicate codes across scopes; Wave 1
				// only seeds site scope so collisions are impossible. Once
				// course/section currencies land, the dispatcher will
				// resolve scope order before calling LoadSnapshot.
				snap.CurrencyByCode[c.Code] = c.ID
			}
		}
		balances, err := deps.Wallet.ListBalancesForUser(ctx, userID)
		if err != nil {
			return snap, fmt.Errorf("LoadSnapshot: list balances: %w", err)
		}
		if len(balances) > 0 {
			snap.WalletBalances = make(map[uint]int64, len(balances))
			for _, b := range balances {
				snap.WalletBalances[b.CurrencyTypeID] = b.Balance
			}
		}
	}

	// Enrollments / LastLogin: deferred to Sprint D. Predicates that read
	// these slices return false-with-reason when they're empty/zero, so
	// leaving them unset here is intentional and safe.

	return snap, nil
}

// outcomeResultValue derives a 0.0–1.0 mastery value from one outcome
// result row. Prefers the explicit Percent column when present; falls
// back to Score / Possible. Returns 0 when neither is usable —
// mastery.LevelFor clamps to "novice" in that case.
func outcomeResultValue(r models.LearningOutcomeResult) float64 {
	if r.Percent != nil {
		return *r.Percent
	}
	if r.Score != nil && r.Possible != nil && *r.Possible > 0 {
		return *r.Score / *r.Possible
	}
	return 0
}

// outcomeResultTime returns the canonical "when did this happen" for an
// outcome result. AssessedAt takes precedence over SubmittedAt because
// the gradebook UI's "as of" column reads AssessedAt.
func outcomeResultTime(r models.LearningOutcomeResult) time.Time {
	if r.AssessedAt != nil {
		return *r.AssessedAt
	}
	if r.SubmittedAt != nil {
		return *r.SubmittedAt
	}
	return time.Time{}
}

// resultIsLater returns true if `cand` should replace `cur` as the
// "latest" outcome result. Prefers AssessedAt/SubmittedAt time when
// both rows carry one; falls back to Attempt number as a stable ordinal
// so latest-selection is deterministic even when timestamps are nil.
func resultIsLater(cand, cur models.LearningOutcomeResult) bool {
	candTime := outcomeResultTime(cand)
	curTime := outcomeResultTime(cur)
	if !candTime.IsZero() && !curTime.IsZero() {
		return candTime.After(curTime)
	}
	if !candTime.IsZero() {
		return true
	}
	if !curTime.IsZero() {
		return false
	}
	return cand.Attempt > cur.Attempt
}
