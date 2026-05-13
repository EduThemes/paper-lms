package effects_test

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
)

// fakeBadgeRepo is the minimum surface area AwardBadge uses.
type fakeBadgeRepo struct {
	rows []models.GamificationBadge
}

func (f *fakeBadgeRepo) Create(_ context.Context, b *models.GamificationBadge) error {
	f.rows = append(f.rows, *b)
	return nil
}
func (f *fakeBadgeRepo) FindByID(_ context.Context, id uint) (*models.GamificationBadge, error) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}
func (f *fakeBadgeRepo) FindByCode(_ context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationBadge, error) {
	for i := range f.rows {
		b := &f.rows[i]
		if b.TenantID == tenantID && b.ScopeType == scopeType && b.ScopeID == scopeID && b.Code == code {
			return b, nil
		}
	}
	return nil, nil
}
func (f *fakeBadgeRepo) Update(_ context.Context, _ *models.GamificationBadge) error { return nil }
func (f *fakeBadgeRepo) Delete(_ context.Context, _ uint) error                       { return nil }
func (f *fakeBadgeRepo) ListByTenant(_ context.Context, _ uint) ([]models.GamificationBadge, error) {
	return f.rows, nil
}

// fakeBadgeAwardRepo records what was awarded and supports idempotency-
// equivalent behavior by deduplicating on (user, badge).
type fakeBadgeAwardRepo struct {
	awards   []models.GamificationBadgeAward
	errOn    bool
	nextID   uint
}

func (f *fakeBadgeAwardRepo) Award(_ context.Context, a *models.GamificationBadgeAward) (bool, error) {
	if f.errOn {
		return false, errors.New("db down")
	}
	for _, ex := range f.awards {
		if ex.UserID == a.UserID && ex.BadgeID == a.BadgeID {
			return false, nil // idempotent — already exists
		}
	}
	f.nextID++
	a.ID = f.nextID
	f.awards = append(f.awards, *a)
	return true, nil
}
func (f *fakeBadgeAwardRepo) Revoke(_ context.Context, _, _ uint) error { return nil }
func (f *fakeBadgeAwardRepo) ListForUser(_ context.Context, userID uint) ([]models.GamificationBadgeAward, error) {
	var out []models.GamificationBadgeAward
	for _, a := range f.awards {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}
func (f *fakeBadgeAwardRepo) FindByUserAndBadge(_ context.Context, userID, badgeID uint) (*models.GamificationBadgeAward, error) {
	for _, a := range f.awards {
		if a.UserID == userID && a.BadgeID == badgeID {
			return &a, nil
		}
	}
	return nil, nil
}

func TestAwardBadge_HappyPath(t *testing.T) {
	repo := &fakeBadgeRepo{rows: []models.GamificationBadge{
		{ID: 100, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "first_quiz"},
	}}
	awards := &fakeBadgeAwardRepo{}

	res, err := effects.AwardBadge{Code: "first_quiz"}.Apply(
		context.Background(),
		effects.EffectDeps{Badge: repo, BadgeAward: awards},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Kind != "AwardBadge" {
		t.Errorf("Kind = %q", res.Kind)
	}
	if len(awards.awards) != 1 {
		t.Fatalf("expected 1 award, got %d", len(awards.awards))
	}
	if awards.awards[0].UserID != 42 || awards.awards[0].BadgeID != 100 {
		t.Errorf("unexpected award: %+v", awards.awards[0])
	}
	if res.Detail["first_time"] != true {
		t.Errorf("Detail.first_time = %v, want true", res.Detail["first_time"])
	}
}

func TestAwardBadge_Idempotent(t *testing.T) {
	// Two fires for the same (user, badge) yield exactly one award row.
	// The second result's Detail.first_time is false so the audit trail
	// records the dedupe rather than silently no-opping.
	repo := &fakeBadgeRepo{rows: []models.GamificationBadge{
		{ID: 100, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "first_quiz"},
	}}
	awards := &fakeBadgeAwardRepo{}

	eff := effects.AwardBadge{Code: "first_quiz"}
	deps := effects.EffectDeps{Badge: repo, BadgeAward: awards}
	trig := effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7}

	first, err := eff.Apply(context.Background(), deps, trig)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	second, err := eff.Apply(context.Background(), deps, trig)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if len(awards.awards) != 1 {
		t.Fatalf("expected 1 award after 2 fires, got %d", len(awards.awards))
	}
	if first.Detail["first_time"] != true {
		t.Errorf("first apply first_time = %v, want true", first.Detail["first_time"])
	}
	if second.Detail["first_time"] != false {
		t.Errorf("second apply first_time = %v, want false (dedupe)", second.Detail["first_time"])
	}
}

func TestAwardBadge_ScopeFallbackFromCourseToSite(t *testing.T) {
	// Badge defined at site only; trigger fires in a course scope.
	repo := &fakeBadgeRepo{rows: []models.GamificationBadge{
		{ID: 100, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "first_quiz"},
	}}
	awards := &fakeBadgeAwardRepo{}

	_, err := effects.AwardBadge{Code: "first_quiz"}.Apply(
		context.Background(),
		effects.EffectDeps{Badge: repo, BadgeAward: awards},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("expected site fallback to succeed, got %v", err)
	}
	if len(awards.awards) != 1 || awards.awards[0].BadgeID != 100 {
		t.Fatalf("expected site-scoped badge (id=100), got %+v", awards.awards)
	}
}

func TestAwardBadge_CourseScopedTakesPrecedenceOverSite(t *testing.T) {
	// Same code at both site and course scope; course wins.
	repo := &fakeBadgeRepo{rows: []models.GamificationBadge{
		{ID: 100, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "explorer"},
		{ID: 200, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, Code: "explorer"},
	}}
	awards := &fakeBadgeAwardRepo{}

	_, err := effects.AwardBadge{Code: "explorer"}.Apply(
		context.Background(),
		effects.EffectDeps{Badge: repo, BadgeAward: awards},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if awards.awards[0].BadgeID != 200 {
		t.Fatalf("expected course-scoped badge (id=200), got id=%d", awards.awards[0].BadgeID)
	}
}

func TestAwardBadge_NotFoundAtAnyScope(t *testing.T) {
	repo := &fakeBadgeRepo{}
	awards := &fakeBadgeAwardRepo{}

	_, err := effects.AwardBadge{Code: "ghost"}.Apply(
		context.Background(),
		effects.EffectDeps{Badge: repo, BadgeAward: awards},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected error when badge undefined")
	}
	if len(awards.awards) != 0 {
		t.Fatalf("expected no awards on resolve failure")
	}
}

func TestAwardBadge_EmptyCode(t *testing.T) {
	_, err := effects.AwardBadge{Code: ""}.Apply(
		context.Background(),
		effects.EffectDeps{Badge: &fakeBadgeRepo{}, BadgeAward: &fakeBadgeAwardRepo{}},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected error on empty Code")
	}
}

func TestAwardBadge_MissingDeps(t *testing.T) {
	_, err := effects.AwardBadge{Code: "x"}.Apply(
		context.Background(),
		effects.EffectDeps{}, // no Badge / BadgeAward
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected error when deps missing")
	}
}
