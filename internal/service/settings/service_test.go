package settings

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ── Test setup ─────────────────────────────────────────────────────

// TestMain sets MFA_ENCRYPTION_KEY before secretbox's sync.Once loads
// the key cache. Settings unit tests round-trip secrets through
// auth.Encrypt / auth.Decrypt, so an unset key would fail every
// secret-typed case.
func TestMain(m *testing.M) {
	key := make([]byte, 32)
	os.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	if err := auth.EnsureKeysLoaded(); err != nil {
		panic("settings test setup: " + err.Error())
	}
	os.Exit(m.Run())
}

// ── In-memory repo ──────────────────────────────────────────────────

type memRepo struct {
	mu   sync.Mutex
	rows map[string]*models.Setting
	seq  uint
}

func newMemRepo() *memRepo { return &memRepo{rows: map[string]*models.Setting{}} }

func keyOf(scopeType string, scopeID uint, key string) string {
	return scopeType + "|" + uintStr(scopeID) + "|" + key
}

func uintStr(u uint) string {
	if u == 0 {
		return "0"
	}
	out := ""
	for u > 0 {
		out = string(rune('0'+u%10)) + out
		u /= 10
	}
	return out
}

func (r *memRepo) FindByScope(ctx context.Context, scopeType string, scopeID uint, key string) (*models.Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if row, ok := r.rows[keyOf(scopeType, scopeID, key)]; ok {
		// Return a copy so the caller can't mutate stored state.
		c := *row
		return &c, nil
	}
	return nil, repository.ErrSettingNotFound
}

func (r *memRepo) ListByScope(ctx context.Context, scopeType string, scopeID uint) ([]models.Setting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prefix := scopeType + "|" + uintStr(scopeID) + "|"
	var out []models.Setting
	for k, row := range r.rows {
		if strings.HasPrefix(k, prefix) {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (r *memRepo) Upsert(ctx context.Context, s *models.Setting) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	s.ID = r.seq
	c := *s
	r.rows[keyOf(s.ScopeType, s.ScopeID, s.Key)] = &c
	return nil
}

func (r *memRepo) Delete(ctx context.Context, scopeType string, scopeID uint, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rows, keyOf(scopeType, scopeID, key))
	return nil
}

// ── Fake audit sink ─────────────────────────────────────────────────

type fakeAuditEvent struct {
	EventType   string
	UserID      uint
	AccountID   *uint
	ContextType string
	Action      string
	Payload     string
}

type fakeAudit struct {
	mu     sync.Mutex
	events []fakeAuditEvent
}

func (f *fakeAudit) LogEvent(ctx context.Context, eventType string, userID uint, courseID, accountID *uint, contextType string, contextID uint, action, payload, ipAddress, userAgent string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, fakeAuditEvent{
		EventType:   eventType,
		UserID:      userID,
		AccountID:   accountID,
		ContextType: contextType,
		Action:      action,
		Payload:     payload,
	})
	return nil
}

// ── Fake account ancestry ───────────────────────────────────────────

type fakeAncestry struct {
	accounts map[uint]*models.Account
}

func newFakeAncestry(parents map[uint]uint) *fakeAncestry {
	a := &fakeAncestry{accounts: map[uint]*models.Account{}}
	for id, parent := range parents {
		acct := &models.Account{ID: id}
		if parent != 0 {
			p := parent
			acct.ParentAccountID = &p
		}
		a.accounts[id] = acct
	}
	return a
}

func (a *fakeAncestry) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	if acct, ok := a.accounts[id]; ok {
		return acct, nil
	}
	return nil, errors.New("account not found")
}

// ── Fixtures ────────────────────────────────────────────────────────

func newServiceWithAudit(t *testing.T, parents map[uint]uint) (*Service, *memRepo, *fakeAudit) {
	t.Helper()
	repo := newMemRepo()
	audit := &fakeAudit{}
	svc := NewService(repo, newFakeAncestry(parents), audit)
	svc.SetEnvReader(func(string) string { return "" })
	return svc, repo, audit
}

// ── Tests ──────────────────────────────────────────────────────────

func TestGet_UnknownKey(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	_, err := svc.Get(context.Background(), "no.such.key", ScopeHints{})
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("expected ErrUnknownKey, got %v", err)
	}
}

func TestGet_DefaultFallback(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	// smtp.port has Default "587" and EnvFallback "SMTP_PORT" — the
	// env-reader fake returns "" so resolution falls all the way
	// through to the default.
	ev, err := svc.Get(context.Background(), "smtp.port", ScopeHints{AccountID: 5})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ev.Source != SourceDefault || ev.Value != "587" || !ev.HasValue {
		t.Fatalf("expected default 587, got %+v", ev)
	}
}

func TestGet_EnvFallback(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	svc.SetEnvReader(func(name string) string {
		if name == "SMTP_HOST" {
			return "mail.example.test"
		}
		return ""
	})
	ev, err := svc.Get(context.Background(), "smtp.host", ScopeHints{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if ev.Source != SourceEnv || ev.Value != "mail.example.test" {
		t.Fatalf("expected env value, got %+v", ev)
	}
}

func TestGet_InstanceBeatsEnvAndDefault(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	svc.SetEnvReader(func(name string) string {
		if name == "SMTP_HOST" {
			return "env.example.test"
		}
		return ""
	})
	if err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "set.example.test", 1); err != nil {
		t.Fatalf("set: %v", err)
	}
	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{})
	if ev.Source != SourceInstance || ev.Value != "set.example.test" {
		t.Fatalf("expected instance override, got %+v", ev)
	}
}

func TestGet_AccountBeatsInstance(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "instance.example", 1)
	_ = svc.Set(context.Background(), ScopeAccount, 42, "smtp.host", "account.example", 1)

	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{AccountID: 42})
	if ev.Source != SourceAccount || ev.Value != "account.example" {
		t.Fatalf("expected account override, got %+v", ev)
	}
	if ev.ScopeID != 42 {
		t.Fatalf("expected scope_id 42, got %d", ev.ScopeID)
	}
}

func TestGet_ParentAccountChain(t *testing.T) {
	// district (1) → school (10) → sub-school (100)
	// Set smtp.host at the district (1); sub-school read should walk
	// 100 → 10 → 1 and pick up the district value.
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{
		1:   0,
		10:  1,
		100: 10,
	})
	_ = svc.Set(context.Background(), ScopeAccount, 1, "smtp.host", "district.example", 1)

	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{AccountID: 100})
	if ev.Source != SourceAccount || ev.Value != "district.example" || ev.ScopeID != 1 {
		t.Fatalf("expected district value via parent walk, got %+v", ev)
	}
}

func TestGet_SubAccountOverridesParent(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{
		1:   0,
		10:  1,
		100: 10,
	})
	_ = svc.Set(context.Background(), ScopeAccount, 1, "smtp.host", "district.example", 1)
	_ = svc.Set(context.Background(), ScopeAccount, 100, "smtp.host", "sub.example", 1)

	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{AccountID: 100})
	if ev.Source != SourceAccount || ev.Value != "sub.example" || ev.ScopeID != 100 {
		t.Fatalf("expected sub-account override, got %+v", ev)
	}
}

func TestGet_ParentChainHandlesCycleSafely(t *testing.T) {
	// 1 → 2 → 1 (operator-induced cycle). Walk must terminate.
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{
		1: 2,
		2: 1,
	})
	ev, err := svc.Get(context.Background(), "smtp.host", ScopeHints{AccountID: 1})
	if err != nil {
		t.Fatalf("get with cyclic parents should not error, got %v", err)
	}
	// No setting anywhere → SourceNone (smtp.host has no Default).
	if ev.Source != SourceNone {
		t.Fatalf("expected SourceNone fallthrough, got %+v", ev)
	}
}

func TestSet_SecretEncryptsOnDisk(t *testing.T) {
	svc, repo, _ := newServiceWithAudit(t, nil)
	if err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.password", "hunter2", 7); err != nil {
		t.Fatalf("set: %v", err)
	}
	row, err := repo.FindByScope(context.Background(), "instance", 0, "smtp.password")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if row.ValuePlain != nil {
		t.Fatalf("secret stored in value_plain: %q — should be NULL", *row.ValuePlain)
	}
	if len(row.ValueEncrypted) == 0 {
		t.Fatal("expected value_encrypted populated for secret")
	}
	// Sanity: the ciphertext must not contain the plaintext string.
	for i := 0; i+len("hunter2") <= len(row.ValueEncrypted); i++ {
		if string(row.ValueEncrypted[i:i+len("hunter2")]) == "hunter2" {
			t.Fatal("plaintext bytes leaked into ciphertext")
		}
	}
}

func TestGet_SecretDecryptsOnRead(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.password", "hunter2", 7)

	ev, err := svc.Get(context.Background(), "smtp.password", ScopeHints{})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !ev.IsSecret {
		t.Fatal("expected IsSecret=true")
	}
	if ev.Value != "hunter2" {
		t.Fatalf("expected decrypted plaintext, got %q", ev.Value)
	}
	if masked := ev.Mask(); masked.Value != "" {
		t.Fatalf("Mask() should strip plaintext, got %q", masked.Value)
	}
}

func TestSet_RejectsUnknownKey(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	err := svc.Set(context.Background(), ScopeInstance, 0, "bogus.key", "value", 1)
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("expected ErrUnknownKey, got %v", err)
	}
}

func TestSet_RejectsScopeNotAllowed(t *testing.T) {
	// storage.s3.bucket is instance-only per the catalog (Wave 4 drop
	// the storage.backend orphan — boot-only settings don't belong in
	// the runtime catalog); account scope should be rejected.
	svc, _, _ := newServiceWithAudit(t, nil)
	err := svc.Set(context.Background(), ScopeAccount, 42, "storage.s3.bucket", "mybucket", 1)
	if !errors.Is(err, ErrScopeNotAllowed) {
		t.Fatalf("expected ErrScopeNotAllowed, got %v", err)
	}
}

func TestSet_RejectsInstanceWithNonZeroScopeID(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	err := svc.Set(context.Background(), ScopeInstance, 5, "smtp.host", "x", 1)
	if !errors.Is(err, ErrScopeNotAllowed) {
		t.Fatalf("expected ErrScopeNotAllowed for instance+nonzero, got %v", err)
	}
}

func TestSet_RejectsAccountWithZeroScopeID(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	err := svc.Set(context.Background(), ScopeAccount, 0, "smtp.host", "x", 1)
	if !errors.Is(err, ErrScopeNotAllowed) {
		t.Fatalf("expected ErrScopeNotAllowed for account+zero, got %v", err)
	}
}

func TestSet_ValidatesIntType(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.port", "not-a-number", 1)
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
}

func TestSet_ValidatesBoolType(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	if err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.enabled", "yeah", 1); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
	if err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.enabled", "true", 1); err != nil {
		t.Fatalf("expected accept for 'true', got %v", err)
	}
}

func TestClear_FallsThroughToParentScope(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{42: 0})
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "instance.example", 1)
	_ = svc.Set(context.Background(), ScopeAccount, 42, "smtp.host", "account.example", 1)

	if err := svc.Clear(context.Background(), ScopeAccount, 42, "smtp.host", 1); err != nil {
		t.Fatalf("clear: %v", err)
	}

	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{AccountID: 42})
	if ev.Source != SourceInstance || ev.Value != "instance.example" {
		t.Fatalf("expected fall-through to instance after clear, got %+v", ev)
	}
}

func TestClear_Idempotent(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	// Never set — Clear should not error.
	if err := svc.Clear(context.Background(), ScopeInstance, 0, "smtp.host", 1); err != nil {
		t.Fatalf("clearing absent key returned %v", err)
	}
}

func TestAuditLog_EmittedOnSet(t *testing.T) {
	svc, _, audit := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "host.example", 42)

	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	e := audit.events[0]
	if e.EventType != "setting_change" || e.Action != "setting.changed" {
		t.Fatalf("event shape: %+v", e)
	}
	if e.UserID != 42 {
		t.Fatalf("expected userID=42, got %d", e.UserID)
	}
	if e.ContextType != "Setting" {
		t.Fatalf("expected context_type=Setting, got %q", e.ContextType)
	}
	// Payload MUST NOT contain the value.
	if strings.Contains(e.Payload, "host.example") {
		t.Fatalf("audit payload leaked value: %q", e.Payload)
	}
}

func TestAuditLog_EmittedOnClear(t *testing.T) {
	svc, _, audit := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "host.example", 42)
	_ = svc.Clear(context.Background(), ScopeInstance, 0, "smtp.host", 42)

	if len(audit.events) != 2 {
		t.Fatalf("expected 2 audit events (set+clear), got %d", len(audit.events))
	}
	if audit.events[1].Action != "setting.cleared" {
		t.Fatalf("expected setting.cleared, got %q", audit.events[1].Action)
	}
}

func TestAuditLog_SecretValueNeverInPayload(t *testing.T) {
	svc, _, audit := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.password", "TopSecret123", 42)

	if len(audit.events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(audit.events))
	}
	if strings.Contains(audit.events[0].Payload, "TopSecret123") {
		t.Fatalf("audit payload leaked secret: %q", audit.events[0].Payload)
	}
}

func TestAuditLog_AccountScopePopulatesAccountID(t *testing.T) {
	svc, _, audit := newServiceWithAudit(t, map[uint]uint{42: 0})
	_ = svc.Set(context.Background(), ScopeAccount, 42, "smtp.host", "x.example", 7)

	if len(audit.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(audit.events))
	}
	if audit.events[0].AccountID == nil || *audit.events[0].AccountID != 42 {
		t.Fatalf("expected accountID pointer to 42, got %v", audit.events[0].AccountID)
	}
}

func TestService_NilAuditNoOps(t *testing.T) {
	repo := newMemRepo()
	svc := NewService(repo, newFakeAncestry(nil), nil)
	svc.SetEnvReader(func(string) string { return "" })

	if err := svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "x", 1); err != nil {
		t.Fatalf("set with nil audit should not error: %v", err)
	}
}

func TestGetEffective_GroupFilter(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, nil)
	out, err := svc.GetEffective(context.Background(), "Email", ScopeHints{})
	if err != nil {
		t.Fatalf("get effective: %v", err)
	}
	// Every Email catalog entry must appear.
	for _, def := range Catalog {
		if def.Group != "Email" {
			continue
		}
		if _, ok := out[def.Key]; !ok {
			t.Errorf("missing key %q in Email group result", def.Key)
		}
	}
	// Non-Email entries must NOT appear.
	for _, def := range Catalog {
		if def.Group == "Email" {
			continue
		}
		if _, ok := out[def.Key]; ok {
			t.Errorf("non-Email key %q leaked into Email-filtered result", def.Key)
		}
	}
}

func TestSet_UpsertOverwritesExisting(t *testing.T) {
	svc, repo, _ := newServiceWithAudit(t, nil)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "first.example", 1)
	_ = svc.Set(context.Background(), ScopeInstance, 0, "smtp.host", "second.example", 1)

	row, _ := repo.FindByScope(context.Background(), "instance", 0, "smtp.host")
	if row.ValuePlain == nil || *row.ValuePlain != "second.example" {
		t.Fatalf("expected upsert to overwrite, got %+v", row.ValuePlain)
	}
}

func TestGet_UserBeatsAccount(t *testing.T) {
	svc, _, _ := newServiceWithAudit(t, map[uint]uint{42: 0})

	// No catalog entry currently allows ScopeUser; verify the
	// resolution skips ScopeUser when the catalog doesn't permit it.
	// This locks the contract — if a future catalog entry adds
	// ScopeUser, the resolution chain will pick it up; today, it
	// falls straight through to account scope.
	_ = svc.Set(context.Background(), ScopeAccount, 42, "smtp.host", "account.example", 1)
	ev, _ := svc.Get(context.Background(), "smtp.host", ScopeHints{UserID: 99, AccountID: 42})
	if ev.Source != SourceAccount {
		t.Fatalf("expected account source (no user-scope entries today), got %+v", ev)
	}
}
