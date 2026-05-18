package gamification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ruleRepoFake satisfies repository.GamificationRuleRepository.
type ruleRepoFake struct {
	rows   map[uint]*models.GamificationRule
	nextID uint
}

func newRuleRepoFake() *ruleRepoFake {
	return &ruleRepoFake{rows: map[uint]*models.GamificationRule{}, nextID: 1}
}

func (f *ruleRepoFake) Create(_ context.Context, r *models.GamificationRule) error {
	r.ID = f.nextID
	f.nextID++
	f.rows[r.ID] = r
	return nil
}
func (f *ruleRepoFake) FindByID(_ context.Context, id uint) (*models.GamificationRule, error) {
	return f.rows[id], nil
}
func (f *ruleRepoFake) Update(_ context.Context, r *models.GamificationRule) error {
	f.rows[r.ID] = r
	return nil
}
func (f *ruleRepoFake) Delete(_ context.Context, id uint) error { delete(f.rows, id); return nil }
func (f *ruleRepoFake) ListEnabledByScope(_ context.Context, _ models.GamificationScopeType, _ uint) ([]models.GamificationRule, error) {
	return nil, nil
}
func (f *ruleRepoFake) ListByTenantID(_ context.Context, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	return &repository.PaginatedResult[models.GamificationRule]{}, nil
}
func (f *ruleRepoFake) ListByScope(_ context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRule], error) {
	var items []models.GamificationRule
	for _, r := range f.rows {
		if r.TenantID == tenantID && r.ScopeType == scopeType && r.ScopeID == scopeID {
			items = append(items, *r)
		}
	}
	return &repository.PaginatedResult[models.GamificationRule]{Items: items, TotalCount: int64(len(items))}, nil
}
func (f *ruleRepoFake) RecordEvaluation(_ context.Context, _ *models.GamificationRuleEvaluation) error {
	return nil
}
func (f *ruleRepoFake) ListEvaluationsForUserRule(_ context.Context, _, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.GamificationRuleEvaluation], error) {
	return &repository.PaginatedResult[models.GamificationRuleEvaluation]{}, nil
}
func (f *ruleRepoFake) LastFiringForUserRule(_ context.Context, _, _ uint) (*models.GamificationRuleEvaluation, error) {
	return nil, nil
}
func (f *ruleRepoFake) CountFiringsInWindow(_ context.Context, _, _ uint, _ time.Time) (int64, error) {
	return 0, nil
}

// Compile-time interface check.
var _ repository.GamificationRuleRepository = (*ruleRepoFake)(nil)

// validRuleInput is a minimal valid recipe.
func validRuleInput() RuleCreateInput {
	return RuleCreateInput{
		Name:          "Pass-the-Quiz Reward",
		AudienceLevel: string(models.AudienceHigherEd),
		Enabled:       true,
		TriggerEvent:  json.RawMessage(`{"kind":"OnEvent","verb":"submitted","object_type":"Submission"}`),
		ConditionSet:  json.RawMessage(`{"kind":"ConditionSet","op":"AND","children":[]}`),
		Effects:       json.RawMessage(`[]`),
	}
}

func TestRuleService_Create_HappyPath(t *testing.T) {
	repo := newRuleRepoFake()
	svc := NewRuleService(repo)
	row, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, validRuleInput())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if row.ID == 0 {
		t.Fatalf("expected ID assigned")
	}
	if row.TenantID != 1 || row.ScopeType != models.ScopeSite || row.ScopeID != 1 {
		t.Fatalf("scope misrouted: got tenant=%d scope=%v id=%d", row.TenantID, row.ScopeType, row.ScopeID)
	}
	if row.CreatedBy == nil || *row.CreatedBy != 7 {
		t.Fatalf("expected CreatedBy=7, got %v", row.CreatedBy)
	}
}

func TestRuleService_Create_RejectsEmptyName(t *testing.T) {
	svc := NewRuleService(newRuleRepoFake())
	in := validRuleInput()
	in.Name = ""
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, in)
	if !errors.Is(err, ErrInvalidRuleName) {
		t.Fatalf("expected ErrInvalidRuleName, got %v", err)
	}
}

func TestRuleService_Create_RejectsBadAudience(t *testing.T) {
	svc := NewRuleService(newRuleRepoFake())
	in := validRuleInput()
	in.AudienceLevel = "nope"
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, in)
	if err == nil {
		t.Fatalf("expected audience validation error")
	}
}

func TestRuleService_Create_RejectsBadTriggerVerb(t *testing.T) {
	svc := NewRuleService(newRuleRepoFake())
	in := validRuleInput()
	in.TriggerEvent = json.RawMessage(`{"kind":"OnEvent","verb":"nope","object_type":"Submission"}`)
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, in)
	if err == nil {
		t.Fatalf("expected trigger validation error")
	}
}

func TestRuleService_Create_RejectsMissingCondition(t *testing.T) {
	svc := NewRuleService(newRuleRepoFake())
	in := validRuleInput()
	in.ConditionSet = nil
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, in)
	if err == nil {
		t.Fatalf("expected condition validation error")
	}
}

func TestRuleService_Get_ScopeMismatch(t *testing.T) {
	repo := newRuleRepoFake()
	svc := NewRuleService(repo)
	row, _ := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, validRuleInput())
	_, err := svc.Get(context.Background(), row.ID, 1, models.ScopeCourse, 99)
	if !errors.Is(err, ErrRuleOutOfScope) {
		t.Fatalf("expected ErrRuleOutOfScope, got %v", err)
	}
}

func TestRuleService_Get_NotFound(t *testing.T) {
	svc := NewRuleService(newRuleRepoFake())
	_, err := svc.Get(context.Background(), 99, 1, models.ScopeSite, 1)
	if !errors.Is(err, ErrRuleNotFound) {
		t.Fatalf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestRuleService_Patch_ClearCooldown(t *testing.T) {
	repo := newRuleRepoFake()
	svc := NewRuleService(repo)
	in := validRuleInput()
	cd := 60
	in.CooldownSeconds = &cd
	row, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, 7, in)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	updated, err := svc.Patch(context.Background(), row.ID, 1, models.ScopeSite, 1, RulePatchInput{ClearCooldown: true})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.CooldownSeconds != nil {
		t.Fatalf("expected cooldown cleared, got %v", *updated.CooldownSeconds)
	}
}
