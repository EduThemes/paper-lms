package repository

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// ErrCurrencyDuplicate is returned by GamificationCurrencyTypeRepository.Create
// when the (tenant_id, scope_type, scope_id, code) tuple already exists. The
// repo translates the unique-constraint hit atomically via
// `INSERT ... ON CONFLICT DO NOTHING RETURNING ...`, so callers can map this
// to a 409 without a two-query pre-check race window.
var ErrCurrencyDuplicate = errors.New("currency with this code already exists in this scope")

// ErrBadgeDuplicate is the W2-D analog of ErrCurrencyDuplicate for the
// (tenant_id, scope_type, scope_id, code) uniqueness constraint on
// gamification_badges. Same atomic INSERT ... ON CONFLICT DO NOTHING
// pattern, same handler→409 translation.
var ErrBadgeDuplicate = errors.New("badge with this code already exists in this scope")

// Phase 6 Wave 1: gamification foundations.
// See docs/research/gamification-2026-05/PHASE6-WAVE1-PLAN.md.

// GamificationEventFilter narrows queries against the xAPI event store.
// Empty fields are ignored; multiple fields AND together.
type GamificationEventFilter struct {
	TenantID     *uint
	ActorID      *uint
	Verb         string
	ObjectType   string
	ObjectID     *uint
	OccurredFrom *time.Time
	OccurredTo   *time.Time
}

type GamificationEventRepository interface {
	Create(ctx context.Context, event *models.GamificationEvent) error
	FindByID(ctx context.Context, id uint) (*models.GamificationEvent, error)
	// FindBySourceEventID supports idempotent ingest of external systems:
	// re-deliveries of the same (source, source_event_id) pair return the
	// original row rather than inserting a duplicate.
	FindBySourceEventID(ctx context.Context, source, sourceEventID string) (*models.GamificationEvent, error)
	List(ctx context.Context, filter GamificationEventFilter, params PaginationParams) (*PaginatedResult[models.GamificationEvent], error)
}

type GamificationRuleRepository interface {
	Create(ctx context.Context, rule *models.GamificationRule) error
	FindByID(ctx context.Context, id uint) (*models.GamificationRule, error)
	Update(ctx context.Context, rule *models.GamificationRule) error
	Delete(ctx context.Context, id uint) error
	// ListEnabledByScope returns enabled rules at the exact (scope_type, scope_id).
	// The dispatch loop (Wave 1 task 10) walks up the org tree itself.
	ListEnabledByScope(ctx context.Context, scopeType models.GamificationScopeType, scopeID uint) ([]models.GamificationRule, error)
	ListByTenantID(ctx context.Context, tenantID uint, params PaginationParams) (*PaginatedResult[models.GamificationRule], error)
	// ListByScope returns every rule (enabled OR disabled) at a precise
	// (tenant, scope_type, scope_id) tuple. Backs the W2-E.1 recipe
	// builder list view — admin sees site rules, instructor sees their
	// own course/section rules, neither sees the other's slice.
	ListByScope(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, params PaginationParams) (*PaginatedResult[models.GamificationRule], error)

	// RecordEvaluation appends an audit row. The (rule_id, user_id, evaluated_at)
	// tuple is uniquely indexed; a same-microsecond duplicate is a bug, not a retry.
	RecordEvaluation(ctx context.Context, eval *models.GamificationRuleEvaluation) error
	ListEvaluationsForUserRule(ctx context.Context, userID, ruleID uint, params PaginationParams) (*PaginatedResult[models.GamificationRuleEvaluation], error)
	// LastFiringForUserRule returns the most recent successful evaluation
	// (result=true) for (rule_id, user_id) — the cooldown check's input.
	// Returns (nil, nil) when the rule has never successfully fired for
	// this user.
	LastFiringForUserRule(ctx context.Context, userID, ruleID uint) (*models.GamificationRuleEvaluation, error)
	// CountFiringsInWindow counts successful evaluations for
	// (rule_id, user_id) since `since`. Powers the max_per_window guard.
	CountFiringsInWindow(ctx context.Context, userID, ruleID uint, since time.Time) (int64, error)
}

// ContentViewRepository persists per-user content-view aggregates that the
// ViewedContent predicate reads at rule-evaluation time. Schema lives at
// migration 000036.
type ContentViewRepository interface {
	// IncrementView upserts the (user, object_type, object_id) row,
	// incrementing view_count and total_seconds and bumping
	// last_viewed_at. Atomic via ON CONFLICT … DO UPDATE.
	IncrementView(ctx context.Context, userID uint, objectType string, objectID uint, durationSeconds int64) error
	// ListByUserAndObjectIDs is the snapshot loader's targeted read.
	ListByUserAndObjectIDs(ctx context.Context, userID uint, objectType string, objectIDs []uint) ([]models.ContentView, error)
	// GetByUserAndObject returns (nil, nil) when no row exists; callers
	// treat that as zero views.
	GetByUserAndObject(ctx context.Context, userID uint, objectType string, objectID uint) (*models.ContentView, error)
}

type GamificationCurrencyTypeRepository interface {
	Create(ctx context.Context, currency *models.GamificationCurrencyType) error
	FindByID(ctx context.Context, id uint) (*models.GamificationCurrencyType, error)
	// FindByCode exact-matches (tenant_id, scope_type, scope_id, code).
	// The resolution-order walk (section → course → school → district → site)
	// is the caller's job; this is the single-lookup primitive.
	FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error)
	Update(ctx context.Context, currency *models.GamificationCurrencyType) error
	Delete(ctx context.Context, id uint) error
	ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error)
	ListInTopbar(ctx context.Context, tenantID uint) ([]models.GamificationCurrencyType, error)
}

// GamificationBadgeRepository persists admin/instructor-authored badge
// definitions. Create returns ErrBadgeDuplicate when the
// (tenant_id, scope_type, scope_id, code) tuple is already taken — the
// translation is atomic at the SQL layer (INSERT ... ON CONFLICT
// DO NOTHING RETURNING).
type GamificationBadgeRepository interface {
	Create(ctx context.Context, badge *models.GamificationBadge) error
	FindByID(ctx context.Context, id uint) (*models.GamificationBadge, error)
	FindByCode(ctx context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationBadge, error)
	Update(ctx context.Context, badge *models.GamificationBadge) error
	Delete(ctx context.Context, id uint) error
	ListByTenant(ctx context.Context, tenantID uint) ([]models.GamificationBadge, error)
}

// GamificationBadgeAwardRepository persists (user, badge) issuances.
// Award is idempotent (atomic via the uniq_gam_badge_award constraint);
// double-awarding the same badge to the same user is a no-op.
type GamificationBadgeAwardRepository interface {
	// Award inserts a (user, badge) row. If the user already holds the
	// badge, the call is a no-op (no error, no duplicate row, no update
	// to AwardedAt). The bool return tells the caller whether a new
	// award actually happened — useful for any future "first time only"
	// emit hook.
	Award(ctx context.Context, award *models.GamificationBadgeAward) (created bool, err error)
	Revoke(ctx context.Context, userID, badgeID uint) error
	ListForUser(ctx context.Context, userID uint) ([]models.GamificationBadgeAward, error)
	FindByUserAndBadge(ctx context.Context, userID, badgeID uint) (*models.GamificationBadgeAward, error)
}

type GamificationWalletRepository interface {
	// GetBalance returns nil (no error) when the (user, currency) pair has
	// never transacted. Callers treat that as a zero balance.
	GetBalance(ctx context.Context, userID, currencyTypeID uint) (*models.GamificationWalletBalance, error)
	ListBalancesForUser(ctx context.Context, userID uint) ([]models.GamificationWalletBalance, error)
	// ApplyTransaction is the single atomic mutation primitive: appends a
	// transaction row and updates the corresponding balance row in one DB
	// transaction. The Wave 1 task-8 AwardCurrency effect calls this.
	ApplyTransaction(ctx context.Context, tx *models.GamificationWalletTransaction) error
	ListTransactionsForUser(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.GamificationWalletTransaction], error)
	// ListTransactionsForUserAndCurrency narrows the ledger to a single
	// currency. Powers the wallet drawer's per-currency tab in Wave 2 —
	// avoids over-fetching when a user has years of cross-currency
	// transactions.
	ListTransactionsForUserAndCurrency(ctx context.Context, userID, currencyTypeID uint, params PaginationParams) (*PaginatedResult[models.GamificationWalletTransaction], error)
	// RankByCurrency (W3-A) returns candidateUserIDs ranked by
	// lifetime_earned DESC for a single currency. Ties resolved by
	// earliest most-recent positive transaction (the earlier-completer
	// ranks higher; doesn't reward sandbagging). Rows with no balance
	// row for this currency surface with lifetime_earned = 0 and rank
	// at the tail.
	//
	// Composition note: callers MUST narrow candidateUserIDs through
	// UserRepository.FilterPublicLeaderboardCandidates first. Opt-out
	// privacy lives in the user repo; this method is rank-only.
	RankByCurrency(ctx context.Context, currencyTypeID uint, candidateUserIDs []uint) ([]RankRow, error)
}

// RankRow is the wallet-repo-level rank tuple. Rank starts at 1.
// LifetimeEarned == 0 for candidates with no balance row in this currency.
type RankRow struct {
	UserID         uint
	LifetimeEarned int64
	Rank           int
}

type GamificationFerpaFieldTagRepository interface {
	Upsert(ctx context.Context, tag *models.GamificationFerpaFieldTag) error
	Find(ctx context.Context, objectType, fieldPath string) (*models.GamificationFerpaFieldTag, error)
	ListByObjectType(ctx context.Context, objectType string) ([]models.GamificationFerpaFieldTag, error)
}

// GamificationLeaderboardSnapshotRepository persists ranked-window
// snapshots (Sprint 7-B). Writes are idempotent via ON CONFLICT DO
// NOTHING on the (scope, currency, window_kind, window_end) UNIQUE
// constraint — the CLI can be re-run for the same window without
// duplicating rows.
type GamificationLeaderboardSnapshotRepository interface {
	// Upsert inserts the snapshot row, no-op on conflict. Returns
	// `created=true` only when a new row was actually written so the
	// CLI can log per-window outcomes accurately.
	Upsert(ctx context.Context, snap *models.GamificationLeaderboardSnapshot) (created bool, err error)
	// FindByWindow returns the snapshot for the exact (scope,
	// currency, kind, end) tuple, or nil if no snapshot exists for
	// that window. The handler uses this to serve `?offset_weeks=N`
	// reads; nil triggers a 404 at the handler.
	FindByWindow(ctx context.Context, scopeType models.GamificationScopeType, scopeID, currencyTypeID uint, kind string, windowEnd time.Time) (*models.GamificationLeaderboardSnapshot, error)
}
