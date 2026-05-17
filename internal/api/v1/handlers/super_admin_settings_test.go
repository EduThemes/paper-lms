package handlers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service/settings"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// ── In-memory SettingRepository ────────────────────────────────────
//
// Settings handler tests don't need the testify mock pattern — they
// want a thin in-memory store the test can preload via real Set calls
// through the service. Same shape used by the service unit tests.

type memSettingRepo struct {
	rows map[string]*models.Setting
	seq  uint
}

func newMemSettingRepo() *memSettingRepo {
	return &memSettingRepo{rows: map[string]*models.Setting{}}
}

func settingKeyOf(scopeType string, scopeID uint, key string) string {
	return scopeType + "|" + uintToStr(scopeID) + "|" + key
}

func uintToStr(u uint) string {
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

func (r *memSettingRepo) FindByScope(ctx context.Context, scopeType string, scopeID uint, key string) (*models.Setting, error) {
	if row, ok := r.rows[settingKeyOf(scopeType, scopeID, key)]; ok {
		c := *row
		return &c, nil
	}
	return nil, repository.ErrSettingNotFound
}

func (r *memSettingRepo) ListByScope(ctx context.Context, scopeType string, scopeID uint) ([]models.Setting, error) {
	var out []models.Setting
	prefix := scopeType + "|" + uintToStr(scopeID) + "|"
	for k, row := range r.rows {
		if strings.HasPrefix(k, prefix) {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (r *memSettingRepo) Upsert(ctx context.Context, s *models.Setting) error {
	r.seq++
	s.ID = r.seq
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	c := *s
	r.rows[settingKeyOf(s.ScopeType, s.ScopeID, s.Key)] = &c
	return nil
}

func (r *memSettingRepo) Delete(ctx context.Context, scopeType string, scopeID uint, key string) error {
	delete(r.rows, settingKeyOf(scopeType, scopeID, key))
	return nil
}

// ── Fake ancestry ────────────────────────────────────────────────────

type fakeAccountAncestry struct {
	parents map[uint]uint
}

func (f *fakeAccountAncestry) FindByID(ctx context.Context, id uint) (*models.Account, error) {
	acct := &models.Account{ID: id}
	if p, ok := f.parents[id]; ok && p != 0 {
		acct.ParentAccountID = &p
	}
	return acct, nil
}

// ── Test fixture ────────────────────────────────────────────────────

type superAdminFixture struct {
	app   *fiber.App
	repo  *memSettingRepo
	svc   *settings.Service
	envFn func(string) string
}

func setupSuperAdminHandler(t *testing.T) *superAdminFixture {
	t.Helper()
	// secretbox bootstraps lazily — set the key before any
	// settings.Service.Set call seeds a secret-typed value.
	key := make([]byte, 32)
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	if err := auth.EnsureKeysLoaded(); err != nil {
		// Already loaded by a previous test in this binary — that's
		// fine, the cached key works the same as a freshly set one
		// (it's all zeros either way, the sync.Once just returns).
		_ = err
	}

	repo := newMemSettingRepo()
	envFn := func(string) string { return "" }
	svc := settings.NewService(repo, &fakeAccountAncestry{parents: map[uint]uint{}}, nil)
	svc.SetEnvReader(envFn)

	handler := handlers.NewSuperAdminSettingsHandler(svc, nil, nil)

	app := testutil.SetupTestApp()
	// Mount routes WITHOUT the auth/RequireSuperAdmin middleware —
	// the handler-level tests verify the handler logic; route-gate
	// integration is tested separately in tenant_isolation_test.go
	// (where RequireSuperAdmin is exercised end-to-end).
	app.Get("/superadmin/settings/groups", handler.Groups)
	app.Get("/superadmin/settings", handler.List)
	app.Get("/superadmin/settings/:key", handler.Get)

	return &superAdminFixture{app: app, repo: repo, svc: svc, envFn: envFn}
}

// ── Tests ──────────────────────────────────────────────────────────

func TestSuperAdminSettings_List_ReturnsAllCatalogEntries(t *testing.T) {
	f := setupSuperAdminHandler(t)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)

	list, ok := body["settings"].([]interface{})
	assert.True(t, ok, "expected settings array")
	assert.Equal(t, len(settings.Catalog), len(list), "every catalog entry should appear")
}

func TestSuperAdminSettings_List_SecretsMasked(t *testing.T) {
	f := setupSuperAdminHandler(t)

	// Seed an instance-scope secret (smtp.password).
	err := f.svc.Set(context.Background(), settings.ScopeInstance, 0, "smtp.password", "hunter2-do-not-leak", 1)
	assert.NoError(t, err)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	list := body["settings"].([]interface{})

	var passwordEntry map[string]interface{}
	for _, item := range list {
		m := item.(map[string]interface{})
		if m["key"] == "smtp.password" {
			passwordEntry = m
			break
		}
	}
	assert.NotNil(t, passwordEntry, "smtp.password entry should be in the list")
	assert.Equal(t, true, passwordEntry["is_secret"])
	assert.Equal(t, "instance", passwordEntry["source"])
	assert.Equal(t, true, passwordEntry["has_value"])
	// CRITICAL: the masked secret MUST NOT echo the plaintext.
	assert.NotContains(t, passwordEntry, "value", "secret value field must be omitted entirely (json omitempty + Mask())")
	// Belt-and-suspenders: scan the entire response for the secret.
	raw := readBody(resp)
	assert.NotContains(t, raw, "hunter2-do-not-leak", "raw response leaked a secret")
}

func TestSuperAdminSettings_List_NonSecretReturnsValue(t *testing.T) {
	f := setupSuperAdminHandler(t)
	err := f.svc.Set(context.Background(), settings.ScopeInstance, 0, "smtp.host", "mail.example.test", 1)
	assert.NoError(t, err)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	raw := readBody(resp)
	assert.Contains(t, raw, "mail.example.test", "non-secret value should be returned")
}

func TestSuperAdminSettings_List_DefaultSource(t *testing.T) {
	f := setupSuperAdminHandler(t)
	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	list := body["settings"].([]interface{})

	// smtp.port has a hard-coded default of "587" and no env-fake
	// override — must show source=default.
	for _, item := range list {
		m := item.(map[string]interface{})
		if m["key"] == "smtp.port" {
			assert.Equal(t, "default", m["source"])
			assert.Equal(t, "587", m["value"])
			return
		}
	}
	t.Fatal("smtp.port not found in response")
}

func TestSuperAdminSettings_Get_UnknownKeyIs404(t *testing.T) {
	f := setupSuperAdminHandler(t)
	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings/no.such.key", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSuperAdminSettings_Get_Secret_Masked(t *testing.T) {
	f := setupSuperAdminHandler(t)
	err := f.svc.Set(context.Background(), settings.ScopeInstance, 0, "smtp.password", "another-secret", 1)
	assert.NoError(t, err)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings/smtp.password", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	raw := readBody(resp)
	assert.NotContains(t, raw, "another-secret", "Get endpoint leaked the secret plaintext")

	body := jsonMapFromString(t, raw)
	assert.Equal(t, true, body["is_secret"])
	assert.Equal(t, true, body["has_value"])
}

func TestSuperAdminSettings_Get_NonSecret_ReturnsValue(t *testing.T) {
	f := setupSuperAdminHandler(t)
	err := f.svc.Set(context.Background(), settings.ScopeInstance, 0, "smtp.host", "mail.example.test", 1)
	assert.NoError(t, err)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings/smtp.host", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "mail.example.test", body["value"])
	assert.Equal(t, false, body["is_secret"])
	assert.Equal(t, "instance", body["source"])
}

func TestSuperAdminSettings_Groups_NoValuesInVocabulary(t *testing.T) {
	f := setupSuperAdminHandler(t)
	// Seed a secret in case any code path accidentally reaches into
	// settings storage when rendering the vocabulary — the response
	// must not contain it.
	err := f.svc.Set(context.Background(), settings.ScopeInstance, 0, "smtp.password", "vocab-test-secret", 1)
	assert.NoError(t, err)

	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings/groups", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	defs, ok := body["definitions"].([]interface{})
	assert.True(t, ok, "expected definitions array")
	assert.Equal(t, len(settings.Catalog), len(defs))

	raw := readBody(resp)
	assert.NotContains(t, raw, "vocab-test-secret", "vocabulary endpoint leaked a live setting value")
	assert.NotContains(t, raw, "\"value\":", "vocabulary entry must not carry a value field")
	assert.NotContains(t, raw, "\"source\":", "vocabulary entry must not carry a source field")
}

func TestSuperAdminSettings_Groups_DefinitionShape(t *testing.T) {
	f := setupSuperAdminHandler(t)
	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings/groups", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, _ := testutil.ParseJSONMap(resp)
	defs := body["definitions"].([]interface{})

	// smtp.password: locks the secret shape.
	// smtp.host:     locks the TestAction wiring (host is the
	//                canonical "trigger field" for the email test).
	sawPassword := false
	sawHost := false
	for _, item := range defs {
		m := item.(map[string]interface{})
		switch m["key"] {
		case "smtp.password":
			assert.Equal(t, "secret", m["value_type"])
			assert.Equal(t, true, m["is_secret"])
			scopes, _ := m["scopes"].([]interface{})
			assert.Contains(t, scopes, "instance")
			assert.Contains(t, scopes, "account")
			sawPassword = true
		case "smtp.host":
			assert.Equal(t, "email", m["test_action"])
			sawHost = true
		}
	}
	assert.True(t, sawPassword, "smtp.password definition not found")
	assert.True(t, sawHost, "smtp.host definition not found")
}

func TestSuperAdminSettings_List_AccountIDHintWalksParentChain(t *testing.T) {
	// district (1) — sub-school (100). Setting at the district is
	// resolved when caller hints account_id=100.
	repo := newMemSettingRepo()
	ancestry := &fakeAccountAncestry{parents: map[uint]uint{
		1:   0,
		100: 1,
	}}
	svc := settings.NewService(repo, ancestry, nil)
	svc.SetEnvReader(func(string) string { return "" })
	err := svc.Set(context.Background(), settings.ScopeAccount, 1, "smtp.host", "district.example", 1)
	assert.NoError(t, err)

	handler := handlers.NewSuperAdminSettingsHandler(svc, nil, nil)
	app := testutil.SetupTestApp()
	app.Get("/superadmin/settings", handler.List)
	app.Get("/superadmin/settings/:key", handler.Get)

	// account_id=100 → resolution walks 100→1 and picks up the district setting.
	resp := testutil.MakeRequest(app, http.MethodGet, "/superadmin/settings/smtp.host?account_id=100", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "district.example", body["value"])
	assert.Equal(t, "account", body["source"])
	assert.Equal(t, float64(1), body["scope_id"], "scope_id should reflect WHERE the value lives, not where the chain started")
}

func TestSuperAdminSettings_List_GarbageAccountIDIgnored(t *testing.T) {
	// account_id=NotAnInt should not crash; should silently behave as if
	// no account hint was provided.
	f := setupSuperAdminHandler(t)
	resp := testutil.MakeRequest(f.app, http.MethodGet, "/superadmin/settings?account_id=junk", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "non-integer account_id must not 500")
}

// jsonMapFromString parses a JSON string into a map. Used after
// readBody() drains the response body — both consume resp.Body, so
// tests that need BOTH a raw scan and structured field access read
// the bytes once and parse from the captured string.
func jsonMapFromString(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("parse json: %v\nbody: %s", err, raw)
	}
	return out
}

// readBody slurps the full response body as a string. Used to scan
// for accidental plaintext leaks across the entire serialized
// response, not just inspectable fields.
func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return string(buf)
}
