package effects_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/gamification/effects"
)

// fakeCurrencyRepo is the minimum surface area AwardCurrency uses.
type fakeCurrencyRepo struct {
	rows []models.GamificationCurrencyType
}

func (f *fakeCurrencyRepo) Create(_ context.Context, c *models.GamificationCurrencyType) error {
	f.rows = append(f.rows, *c)
	return nil
}
func (f *fakeCurrencyRepo) FindByID(_ context.Context, id uint) (*models.GamificationCurrencyType, error) {
	for i := range f.rows {
		if f.rows[i].ID == id {
			return &f.rows[i], nil
		}
	}
	return nil, nil
}
func (f *fakeCurrencyRepo) FindByCode(_ context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error) {
	for i := range f.rows {
		c := &f.rows[i]
		if c.TenantID == tenantID && c.ScopeType == scopeType && c.ScopeID == scopeID && c.Code == code {
			return c, nil
		}
	}
	return nil, nil
}
func (f *fakeCurrencyRepo) Update(_ context.Context, c *models.GamificationCurrencyType) error {
	for i := range f.rows {
		if f.rows[i].ID == c.ID {
			f.rows[i] = *c
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeCurrencyRepo) Delete(_ context.Context, id uint) error {
	for i := range f.rows {
		if f.rows[i].ID == id {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}
func (f *fakeCurrencyRepo) ListByTenant(_ context.Context, _ uint) ([]models.GamificationCurrencyType, error) {
	return f.rows, nil
}
func (f *fakeCurrencyRepo) ListInTopbar(_ context.Context, _ uint) ([]models.GamificationCurrencyType, error) {
	return f.rows, nil
}

// fakeWalletRepo captures ApplyTransaction calls for assertions; injects an
// error if errOnApply is non-nil.
type fakeWalletRepo struct {
	applied    []models.GamificationWalletTransaction
	errOnApply error
}

func (f *fakeWalletRepo) GetBalance(_ context.Context, _, _ uint) (*models.GamificationWalletBalance, error) {
	return nil, nil
}
func (f *fakeWalletRepo) ListBalancesForUser(_ context.Context, _ uint) ([]models.GamificationWalletBalance, error) {
	return nil, nil
}
func (f *fakeWalletRepo) ApplyTransaction(_ context.Context, tx *models.GamificationWalletTransaction) error {
	if f.errOnApply != nil {
		return f.errOnApply
	}
	f.applied = append(f.applied, *tx)
	return nil
}
func (f *fakeWalletRepo) ListTransactionsForUser(_ context.Context, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.GamificationWalletTransaction], error) {
	return nil, nil
}

func ptrFloat(f float64) *float64 { return &f }

func TestAwardCurrency_HappyPath_SiteScoped(t *testing.T) {
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "xp", FerpaClassification: "non_PII"},
	}}
	wal := &fakeWalletRepo{}

	res, err := effects.AwardCurrency{Code: "xp", Amount: 50}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Kind != "AwardCurrency" {
		t.Errorf("Kind = %q", res.Kind)
	}
	if len(wal.applied) != 1 {
		t.Fatalf("expected 1 transaction applied, got %d", len(wal.applied))
	}
	tx := wal.applied[0]
	if tx.Delta != 50 || tx.UserID != 42 || tx.CurrencyTypeID != 11 {
		t.Errorf("unexpected tx: %+v", tx)
	}
	if tx.Reason != "rule:7" {
		t.Errorf("Reason = %q, want rule:7", tx.Reason)
	}
	if tx.TriggeringRuleID == nil || *tx.TriggeringRuleID != 7 {
		t.Errorf("TriggeringRuleID = %v", tx.TriggeringRuleID)
	}
	if len(tx.PolicyFlags) != 0 {
		t.Errorf("non_PII currency should have no policy flags, got %v", []string(tx.PolicyFlags))
	}
}

func TestAwardCurrency_ScopeFallbackFromCourseToSite(t *testing.T) {
	// xp defined at site only; trigger fires in a course scope.
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "xp", FerpaClassification: "non_PII"},
	}}
	wal := &fakeWalletRepo{}

	_, err := effects.AwardCurrency{Code: "xp", Amount: 10}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("expected site fallback to succeed, got %v", err)
	}
	if len(wal.applied) != 1 || wal.applied[0].CurrencyTypeID != 11 {
		t.Fatalf("expected site-scoped xp (id=11) to be used; got %+v", wal.applied)
	}
}

func TestAwardCurrency_CourseScopedTakesPrecedenceOverSite(t *testing.T) {
	// coins defined at both site and course; course should win.
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "coins", FerpaClassification: "non_PII"},
		{ID: 22, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, Code: "coins", FerpaClassification: "non_PII"},
	}}
	wal := &fakeWalletRepo{}

	_, err := effects.AwardCurrency{Code: "coins", Amount: 10}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeCourse, ScopeID: 99, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wal.applied[0].CurrencyTypeID != 22 {
		t.Fatalf("expected course-scoped coins (id=22), got id=%d", wal.applied[0].CurrencyTypeID)
	}
}

func TestAwardCurrency_NotFoundAtAnyScope(t *testing.T) {
	cur := &fakeCurrencyRepo{}
	wal := &fakeWalletRepo{}

	_, err := effects.AwardCurrency{Code: "gems", Amount: 1}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected error when currency not defined")
	}
	if len(wal.applied) != 0 {
		t.Fatalf("expected no transactions applied on resolve failure")
	}
}

func TestAwardCurrency_MultiplierApplied(t *testing.T) {
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "xp", FerpaClassification: "non_PII"},
	}}
	wal := &fakeWalletRepo{}

	_, err := effects.AwardCurrency{Code: "xp", Amount: 10, Multiplier: ptrFloat(2.5)}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wal.applied[0].Delta != 25 { // round(10 × 2.5) = 25
		t.Errorf("expected delta 25, got %d", wal.applied[0].Delta)
	}
}

func TestAwardCurrency_EducationRecordPolicyFlags(t *testing.T) {
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "mastery_points", FerpaClassification: "education_record"},
	}}
	wal := &fakeWalletRepo{}

	_, err := effects.AwardCurrency{Code: "mastery_points", Amount: 5}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := []string(wal.applied[0].PolicyFlags)
	want := []string{"education_record", "ferpa_protected"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PolicyFlags = %v, want %v", got, want)
	}
}

func TestAwardCurrency_RejectsNonPositiveAmount(t *testing.T) {
	cur := &fakeCurrencyRepo{}
	wal := &fakeWalletRepo{}
	_, err := effects.AwardCurrency{Code: "xp", Amount: 0}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected error for Amount=0")
	}
}

func TestAwardCurrency_PropagatesWalletError(t *testing.T) {
	cur := &fakeCurrencyRepo{rows: []models.GamificationCurrencyType{
		{ID: 11, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, Code: "xp", FerpaClassification: "non_PII"},
	}}
	wal := &fakeWalletRepo{errOnApply: errors.New("balance exhausted")}

	_, err := effects.AwardCurrency{Code: "xp", Amount: 10}.Apply(
		context.Background(),
		effects.EffectDeps{Wallet: wal, CurrencyType: cur},
		effects.TriggeringContext{ActorID: 42, TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1, RuleID: 7},
	)
	if err == nil {
		t.Fatalf("expected wallet error to propagate")
	}
}
