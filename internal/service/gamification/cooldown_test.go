package gamification_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification"
)

// fakeRuleRepo is a DB-free fake for repository.GamificationRuleRepository.
// Only LastFiringForUserRule and CountFiringsInWindow have meaningful
// behavior; the rest of the interface is satisfied with panic stubs to
// catch accidental dispatcher calls during cooldown evaluation.
type fakeRuleRepo struct {
	lastFiring    *models.GamificationRuleEvaluation
	lastFiringErr error
	countInWindow int64
	countErr      error

	// observed inputs for assertion
	lastSeenSince time.Time
}

func (f *fakeRuleRepo) LastFiringForUserRule(_ context.Context, _, _ uint) (*models.GamificationRuleEvaluation, error) {
	return f.lastFiring, f.lastFiringErr
}

func (f *fakeRuleRepo) CountFiringsInWindow(_ context.Context, _, _ uint, since time.Time) (int64, error) {
	f.lastSeenSince = since
	return f.countInWindow, f.countErr
}

// --- panic stubs for the rest of GamificationRuleRepository ---

func (f *fakeRuleRepo) Create(context.Context, *models.GamificationRule) error {
	panic("fakeRuleRepo.Create: not expected to be called")
}
func (f *fakeRuleRepo) FindByID(context.Context, uint) (*models.GamificationRule, error) {
	panic("fakeRuleRepo.FindByID: not expected to be called")
}
func (f *fakeRuleRepo) Update(context.Context, *models.GamificationRule) error {
	panic("fakeRuleRepo.Update: not expected to be called")
}
func (f *fakeRuleRepo) Delete(context.Context, uint) error {
	panic("fakeRuleRepo.Delete: not expected to be called")
}
func (f *fakeRuleRepo) ListEnabledByScope(context.Context, models.GamificationScopeType, uint) ([]models.GamificationRule, error) {
	panic("fakeRuleRepo.ListEnabledByScope: not expected to be called")
}
func (f *fakeRuleRepo) ListByTenantID(context.Context, uint, repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	panic("fakeRuleRepo.ListByTenantID: not expected to be called")
}
func (f *fakeRuleRepo) RecordEvaluation(context.Context, *models.GamificationRuleEvaluation) error {
	panic("fakeRuleRepo.RecordEvaluation: not expected to be called")
}
func (f *fakeRuleRepo) ListEvaluationsForUserRule(context.Context, uint, uint, repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRuleEvaluation], error) {
	panic("fakeRuleRepo.ListEvaluationsForUserRule: not expected to be called")
}

// Compile-time assertion that fakeRuleRepo satisfies the interface
// CheckCooldown depends on. If the interface grows a new method, this
// blows up at build time rather than at runtime in a panic stub.
var _ repository.GamificationRuleRepository = (*fakeRuleRepo)(nil)

// ptrInt is a tiny helper for the *int cooldown field.
func ptrInt(v int) *int { return &v }

// mustJSON marshals a value as datatypes.JSON or fails the test.
func mustJSON(t *testing.T, v any) datatypes.JSON {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return datatypes.JSON(b)
}

func TestCheckCooldown(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	type setup struct {
		rule          *models.GamificationRule
		lastFiring    *models.GamificationRuleEvaluation
		countInWindow int64
	}

	cases := []struct {
		name        string
		setup       setup
		wantAllowed bool
		wantReason  string // substring match; empty means any
		wantErr     bool
	}{
		{
			name: "no cooldown, no max-per-window → allowed",
			setup: setup{
				rule: &models.GamificationRule{ID: 1},
			},
			wantAllowed: true,
		},
		{
			name: "cooldown set, no prior firing → allowed",
			setup: setup{
				rule:       &models.GamificationRule{ID: 1, CooldownSeconds: ptrInt(60)},
				lastFiring: nil,
			},
			wantAllowed: true,
		},
		{
			name: "cooldown active, last firing inside window → blocked",
			setup: setup{
				rule: &models.GamificationRule{ID: 1, CooldownSeconds: ptrInt(60)},
				lastFiring: &models.GamificationRuleEvaluation{
					EvaluatedAt: now.Add(-30 * time.Second),
				},
			},
			wantAllowed: false,
			wantReason:  "cooldown active",
		},
		{
			name: "cooldown active, last firing past window → allowed",
			setup: setup{
				rule: &models.GamificationRule{ID: 1, CooldownSeconds: ptrInt(60)},
				lastFiring: &models.GamificationRuleEvaluation{
					EvaluatedAt: now.Add(-120 * time.Second),
				},
			},
			wantAllowed: true,
		},
		{
			name: "cooldown <= 0 is skipped even with recent firing",
			setup: setup{
				rule: &models.GamificationRule{ID: 1, CooldownSeconds: ptrInt(0)},
				lastFiring: &models.GamificationRuleEvaluation{
					EvaluatedAt: now.Add(-1 * time.Second),
				},
			},
			wantAllowed: true,
		},
		{
			name: "max_per_window day, count under limit → allowed",
			setup: setup{
				rule: &models.GamificationRule{
					ID:           1,
					MaxPerWindow: mustJSON(t, map[string]any{"window": "day", "count": 3}),
				},
				countInWindow: 2,
			},
			wantAllowed: true,
		},
		{
			name: "max_per_window day, count at limit → blocked",
			setup: setup{
				rule: &models.GamificationRule{
					ID:           1,
					MaxPerWindow: mustJSON(t, map[string]any{"window": "day", "count": 3}),
				},
				countInWindow: 3,
			},
			wantAllowed: false,
			wantReason:  "max_per_window reached (3 in day)",
		},
		{
			name: "max_per_window week, count over limit → blocked",
			setup: setup{
				rule: &models.GamificationRule{
					ID:           1,
					MaxPerWindow: mustJSON(t, map[string]any{"window": "week", "count": 5}),
				},
				countInWindow: 9,
			},
			wantAllowed: false,
			wantReason:  "max_per_window reached (5 in week)",
		},
		{
			name: "max_per_window lifetime, zero count → allowed",
			setup: setup{
				rule: &models.GamificationRule{
					ID:           1,
					MaxPerWindow: mustJSON(t, map[string]any{"window": "lifetime", "count": 1}),
				},
				countInWindow: 0,
			},
			wantAllowed: true,
		},
		{
			name: "unknown window string → error",
			setup: setup{
				rule: &models.GamificationRule{
					ID:           1,
					MaxPerWindow: mustJSON(t, map[string]any{"window": "fortnight", "count": 1}),
				},
			},
			wantErr: true,
		},
		{
			name: "cooldown allows but max_per_window blocks → blocked by max_per_window",
			setup: setup{
				rule: &models.GamificationRule{
					ID:              1,
					CooldownSeconds: ptrInt(60),
					MaxPerWindow:    mustJSON(t, map[string]any{"window": "day", "count": 2}),
				},
				lastFiring: &models.GamificationRuleEvaluation{
					EvaluatedAt: now.Add(-10 * time.Minute),
				},
				countInWindow: 2,
			},
			wantAllowed: false,
			wantReason:  "max_per_window",
		},
		{
			name: "both gates pass → allowed",
			setup: setup{
				rule: &models.GamificationRule{
					ID:              1,
					CooldownSeconds: ptrInt(60),
					MaxPerWindow:    mustJSON(t, map[string]any{"window": "day", "count": 5}),
				},
				lastFiring: &models.GamificationRuleEvaluation{
					EvaluatedAt: now.Add(-10 * time.Minute),
				},
				countInWindow: 1,
			},
			wantAllowed: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRuleRepo{
				lastFiring:    tc.setup.lastFiring,
				countInWindow: tc.setup.countInWindow,
			}
			got, err := gamification.CheckCooldown(context.Background(), repo, tc.setup.rule, 42, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got result %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Allowed != tc.wantAllowed {
				t.Errorf("Allowed = %v, want %v (reason: %q)", got.Allowed, tc.wantAllowed, got.Reason)
			}
			if !tc.wantAllowed && tc.wantReason != "" && !strings.Contains(got.Reason, tc.wantReason) {
				t.Errorf("Reason = %q, want substring %q", got.Reason, tc.wantReason)
			}
			if tc.wantAllowed && got.Reason != "" {
				t.Errorf("Reason should be empty when Allowed; got %q", got.Reason)
			}
		})
	}
}

// TestCheckCooldown_RepoErrorPropagation ensures repo errors surface as
// CheckCooldown errors rather than being swallowed into Allowed=false.
func TestCheckCooldown_RepoErrorPropagation(t *testing.T) {
	now := time.Now()

	t.Run("last-firing error", func(t *testing.T) {
		repo := &fakeRuleRepo{lastFiringErr: errors.New("db boom")}
		rule := &models.GamificationRule{ID: 1, CooldownSeconds: ptrInt(60)}
		_, err := gamification.CheckCooldown(context.Background(), repo, rule, 1, now)
		if err == nil {
			t.Fatal("expected error from LastFiringForUserRule failure")
		}
	})

	t.Run("count error", func(t *testing.T) {
		repo := &fakeRuleRepo{countErr: errors.New("db boom")}
		rule := &models.GamificationRule{
			ID:           1,
			MaxPerWindow: mustJSON(t, map[string]any{"window": "day", "count": 1}),
		}
		_, err := gamification.CheckCooldown(context.Background(), repo, rule, 1, now)
		if err == nil {
			t.Fatal("expected error from CountFiringsInWindow failure")
		}
	})
}

// TestCheckCooldown_WindowStart asserts the rolling-window math is the
// documented one (now - 24h for "day", zero-time for "lifetime").
func TestCheckCooldown_WindowStart(t *testing.T) {
	now := time.Date(2026, 5, 12, 12, 0, 0, 0, time.UTC)

	t.Run("day is rolling 24h", func(t *testing.T) {
		repo := &fakeRuleRepo{countInWindow: 0}
		rule := &models.GamificationRule{
			ID:           1,
			MaxPerWindow: mustJSON(t, map[string]any{"window": "day", "count": 5}),
		}
		if _, err := gamification.CheckCooldown(context.Background(), repo, rule, 1, now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := now.Add(-24 * time.Hour)
		if !repo.lastSeenSince.Equal(want) {
			t.Errorf("CountFiringsInWindow since = %v, want %v", repo.lastSeenSince, want)
		}
	})

	t.Run("lifetime is zero time", func(t *testing.T) {
		repo := &fakeRuleRepo{countInWindow: 0}
		rule := &models.GamificationRule{
			ID:           1,
			MaxPerWindow: mustJSON(t, map[string]any{"window": "lifetime", "count": 5}),
		}
		if _, err := gamification.CheckCooldown(context.Background(), repo, rule, 1, now); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !repo.lastSeenSince.IsZero() {
			t.Errorf("CountFiringsInWindow since = %v, want zero time", repo.lastSeenSince)
		}
	})
}
