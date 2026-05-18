package gamification

import (
	"context"
	"errors"
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// fakeCurrencyRepo is a hand-rolled fake satisfying
// repository.GamificationCurrencyTypeRepository. Mock-fakes are preferred
// over testify Mock per CLAUDE.md (memory: feedback_audit_after_each_wave).
type fakeCurrencyRepo struct {
	rows           map[uint]*models.GamificationCurrencyType
	nextID         uint
	createErr      error
	updateErr      error
	findErr        error
	listErr        error
	listInTopbarOK bool
}

func newFakeCurrencyRepo() *fakeCurrencyRepo {
	return &fakeCurrencyRepo{rows: map[uint]*models.GamificationCurrencyType{}, nextID: 1}
}

func (f *fakeCurrencyRepo) Create(_ context.Context, c *models.GamificationCurrencyType) error {
	if f.createErr != nil {
		return f.createErr
	}
	// Naive duplicate detection on (tenant, scope_type, scope_id, code).
	for _, r := range f.rows {
		if r.TenantID == c.TenantID && r.ScopeType == c.ScopeType && r.ScopeID == c.ScopeID && r.Code == c.Code {
			return repository.ErrCurrencyDuplicate
		}
	}
	c.ID = f.nextID
	f.nextID++
	f.rows[c.ID] = c
	return nil
}

func (f *fakeCurrencyRepo) FindByID(_ context.Context, id uint) (*models.GamificationCurrencyType, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.rows[id], nil
}

func (f *fakeCurrencyRepo) FindByCode(_ context.Context, tenantID uint, scopeType models.GamificationScopeType, scopeID uint, code string) (*models.GamificationCurrencyType, error) {
	for _, r := range f.rows {
		if r.TenantID == tenantID && r.ScopeType == scopeType && r.ScopeID == scopeID && r.Code == code {
			return r, nil
		}
	}
	return nil, nil
}

func (f *fakeCurrencyRepo) Update(_ context.Context, c *models.GamificationCurrencyType) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if _, ok := f.rows[c.ID]; !ok {
		return errors.New("not found")
	}
	f.rows[c.ID] = c
	return nil
}

func (f *fakeCurrencyRepo) Delete(_ context.Context, id uint) error {
	delete(f.rows, id)
	return nil
}

func (f *fakeCurrencyRepo) ListByTenant(_ context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	var out []models.GamificationCurrencyType
	for _, r := range f.rows {
		if r.TenantID == tenantID {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeCurrencyRepo) ListInTopbar(_ context.Context, tenantID uint) ([]models.GamificationCurrencyType, error) {
	f.listInTopbarOK = true
	var out []models.GamificationCurrencyType
	for _, r := range f.rows {
		if r.TenantID == tenantID && r.VisibleInTopbar {
			out = append(out, *r)
		}
	}
	return out, nil
}

func TestCurrencyService_Create_HappyPath(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	row, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code:         "coins",
		DisplayLabel: "Coin",
		Spendable:    true,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if row.ID == 0 {
		t.Fatalf("expected ID assignment")
	}
	if row.SystemOwned {
		t.Fatalf("expected system_owned=false on create")
	}
	if row.FerpaClassification != "non_PII" {
		t.Fatalf("expected non_PII default, got %q", row.FerpaClassification)
	}
}

func TestCurrencyService_Create_RejectsBadCode(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	for _, code := range []string{"", "X", "1coin", "has space"} {
		_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
			Code: code, DisplayLabel: "Test",
		})
		if !errors.Is(err, ErrInvalidCurrencyCode) {
			t.Fatalf("code %q: expected ErrInvalidCurrencyCode, got %v", code, err)
		}
	}
}

func TestCurrencyService_Create_RejectsEmptyLabel(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code: "coins", DisplayLabel: "",
	})
	if !errors.Is(err, ErrInvalidLabel) {
		t.Fatalf("expected ErrInvalidLabel, got %v", err)
	}
}

func TestCurrencyService_Create_RejectsBadColor(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code: "coins", DisplayLabel: "Coin", Color: "not-a-hex",
	})
	if !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("expected ErrInvalidColor, got %v", err)
	}
}

func TestCurrencyService_Create_Duplicate(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	_, err := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code: "coins", DisplayLabel: "Coin",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	_, err = svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code: "coins", DisplayLabel: "Coin2",
	})
	if !errors.Is(err, repository.ErrCurrencyDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestCurrencyService_Update_ScopeMismatch(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	row, _ := svc.Create(context.Background(), 1, models.ScopeSite, 1, CurrencyCreateInput{
		Code: "coins", DisplayLabel: "Coin",
	})
	// Try to update via a different scope (course/99).
	label := "Hijack"
	_, err := svc.Update(context.Background(), row.ID, 1, models.ScopeCourse, 99, CurrencyPatchInput{
		DisplayLabel: &label,
	})
	if !errors.Is(err, ErrCurrencyOutOfScope) {
		t.Fatalf("expected ErrCurrencyOutOfScope, got %v", err)
	}
}

func TestCurrencyService_Delete_SystemOwned(t *testing.T) {
	repo := newFakeCurrencyRepo()
	row := &models.GamificationCurrencyType{
		TenantID: 1, ScopeType: models.ScopeSite, ScopeID: 1,
		Code: "xp", DisplayLabel: "XP", SystemOwned: true,
	}
	_ = repo.Create(context.Background(), row)
	svc := NewCurrencyService(repo)
	err := svc.Delete(context.Background(), row.ID, 1, models.ScopeSite, 1)
	if !errors.Is(err, ErrSystemCurrencyImmutable) {
		t.Fatalf("expected ErrSystemCurrencyImmutable, got %v", err)
	}
}

func TestCurrencyService_List_TopbarOnlyDelegatesToRepo(t *testing.T) {
	repo := newFakeCurrencyRepo()
	svc := NewCurrencyService(repo)
	_, _ = svc.List(context.Background(), 1, true)
	if !repo.listInTopbarOK {
		t.Fatalf("expected ListInTopbar to be called")
	}
}
