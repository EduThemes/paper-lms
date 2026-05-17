package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
)

// fakeAuthProviderRepo is a minimal in-memory implementation of
// repository.AuthenticationProviderRepository for handler tests. We
// only exercise Create/FindByID/Update — the rest return zero values.
type fakeAuthProviderRepo struct {
	mu      sync.Mutex
	byID    map[uint]*models.AuthenticationProvider
	created []*models.AuthenticationProvider
	updated []*models.AuthenticationProvider
	nextID  uint
}

func newFakeAuthProviderRepo() *fakeAuthProviderRepo {
	return &fakeAuthProviderRepo{
		byID:   map[uint]*models.AuthenticationProvider{},
		nextID: 1,
	}
}

func (f *fakeAuthProviderRepo) Create(ctx context.Context, p *models.AuthenticationProvider) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p.ID = f.nextID
	f.nextID++
	// Copy into the map so later mutation by the handler doesn't shadow
	// what landed at "persistence" time.
	stored := *p
	f.byID[p.ID] = &stored
	f.created = append(f.created, &stored)
	return nil
}

func (f *fakeAuthProviderRepo) FindByID(ctx context.Context, id uint) (*models.AuthenticationProvider, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return nil, repositoryNotFound()
	}
	// Return a copy — service layer mutates the returned struct.
	out := *p
	return &out, nil
}

func (f *fakeAuthProviderRepo) Update(ctx context.Context, p *models.AuthenticationProvider) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := *p
	f.byID[p.ID] = &stored
	f.updated = append(f.updated, &stored)
	return nil
}

func (f *fakeAuthProviderRepo) Delete(ctx context.Context, id uint) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *fakeAuthProviderRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuthenticationProvider], error) {
	return &repository.PaginatedResult[models.AuthenticationProvider]{}, nil
}

func (f *fakeAuthProviderRepo) FindByAccountAndType(ctx context.Context, accountID uint, authType string) ([]models.AuthenticationProvider, error) {
	return nil, nil
}

// repositoryNotFound returns a sentinel mirroring the gorm.ErrRecordNotFound
// the real Postgres repo returns. Service layer only checks `err != nil`,
// so any non-nil error is sufficient.
func repositoryNotFound() error {
	return &notFoundErr{}
}

type notFoundErr struct{}

func (n *notFoundErr) Error() string { return "record not found" }

// setEncryptionKeyForTest mirrors the auth package's setKey helper —
// duplicated here because that helper is unexported. We provision a
// 32-byte AES-256 key and reset the auth package's sync.Once cache by
// calling EnsureKeysLoaded after t.Setenv.
func setEncryptionKeyForTest(t *testing.T) {
	t.Helper()
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	// EnsureKeysLoaded is sync.Once-protected; if another test in the
	// same binary already loaded a (possibly different) key, the load
	// is cached. The auth package's internal resetKeys() isn't visible
	// here. We work around this by encrypting with the key the auth
	// package already has and decrypting via the same package — both
	// directions share the cache, so the test is self-consistent even
	// when the key wasn't this test's t.Setenv.
	_ = auth.EnsureKeysLoaded()
}

// TestCreateProvider_EncryptsLDAPBindPassword is the load-bearing
// regression test for the Phase 9-PRE encryption-at-rest contract.
// CreateProvider must:
//
//  1. Encrypt the inbound plaintext LDAP bind password via
//     secretbox.Encrypt and store it in LDAPBindPasswordEncrypted.
//  2. Blank the legacy LDAPBindPassword plaintext field so no new row
//     ever lands on disk with the plaintext column populated.
//  3. Decrypt cleanly back to the original plaintext via auth.Decrypt.
func TestCreateProvider_EncryptsLDAPBindPassword(t *testing.T) {
	setEncryptionKeyForTest(t)

	repo := newFakeAuthProviderRepo()
	svc := service.NewAuthProviderService(repo)
	h := NewAuthProviderHandler(svc)

	app := fiber.New()
	app.Post("/accounts/:account_id/authentication_providers", h.CreateProvider)

	body := map[string]any{
		"auth_type":          "ldap",
		"ldap_host":          "ldap.example.com",
		"ldap_port":          389,
		"ldap_base":          "dc=example,dc=com",
		"ldap_bind_dn":       "cn=service,dc=example,dc=com",
		"ldap_bind_password": "supersecret",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/accounts/1/authentication_providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 201; body=%s", resp.StatusCode, respBody)
	}

	if len(repo.created) != 1 {
		t.Fatalf("expected 1 row persisted, got %d", len(repo.created))
	}
	stored := repo.created[0]

	// (1) Plaintext column MUST be blank on the persisted row.
	if stored.LDAPBindPassword != "" {
		t.Errorf("plaintext LDAPBindPassword leaked into persisted row: got %q, want empty", stored.LDAPBindPassword)
	}

	// (2) Encrypted column MUST be populated.
	if len(stored.LDAPBindPasswordEncrypted) == 0 {
		t.Fatal("LDAPBindPasswordEncrypted is empty — secretbox.Encrypt was not invoked on the create path")
	}

	// (3) Bytes MUST NOT equal the plaintext (regression guard against
	// accidentally storing plaintext into the encrypted column).
	if bytes.Equal(stored.LDAPBindPasswordEncrypted, []byte("supersecret")) {
		t.Error("LDAPBindPasswordEncrypted bytes equal the plaintext — encryption is a no-op")
	}

	// (4) Decrypt round-trip MUST recover the original plaintext.
	pt, err := auth.Decrypt(stored.LDAPBindPasswordEncrypted)
	if err != nil {
		t.Fatalf("decrypt persisted ciphertext: %v", err)
	}
	if string(pt) != "supersecret" {
		t.Errorf("decrypt round-trip: got %q, want %q", pt, "supersecret")
	}
}

// TestCreateProvider_NonLDAPDoesNotTouchLDAPColumns sanity-checks that
// the encryption branch is gated by auth_type — creating a SAML or
// OIDC provider with no LDAP fields produces a row with both LDAP
// columns empty.
func TestCreateProvider_NonLDAPDoesNotTouchLDAPColumns(t *testing.T) {
	setEncryptionKeyForTest(t)

	repo := newFakeAuthProviderRepo()
	svc := service.NewAuthProviderService(repo)
	h := NewAuthProviderHandler(svc)

	app := fiber.New()
	app.Post("/accounts/:account_id/authentication_providers", h.CreateProvider)

	body := map[string]any{
		"auth_type":     "saml",
		"idp_entity_id": "https://idp.example.com/metadata",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/accounts/1/authentication_providers", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 201; body=%s", resp.StatusCode, respBody)
	}

	stored := repo.created[0]
	if stored.LDAPBindPassword != "" || len(stored.LDAPBindPasswordEncrypted) > 0 {
		t.Errorf("non-LDAP provider has LDAP password columns set: plaintext=%q, encrypted_len=%d",
			stored.LDAPBindPassword, len(stored.LDAPBindPasswordEncrypted))
	}
}

// TestUpdateProvider_EncryptsRotatedLDAPBindPassword covers the
// rotation path: an admin edits an existing LDAP provider and supplies
// a new bind password. The service must seal the new value into the
// encrypted column and blank the plaintext field on the stored row.
// A subsequent update that leaves the field blank must NOT clobber the
// existing ciphertext.
func TestUpdateProvider_EncryptsRotatedLDAPBindPassword(t *testing.T) {
	setEncryptionKeyForTest(t)

	// Seed an existing LDAP provider with a stale plaintext password
	// (the pre-Phase-9-PRE row shape).
	repo := newFakeAuthProviderRepo()
	existing := &models.AuthenticationProvider{
		AccountID:        1,
		AuthType:         "ldap",
		LDAPHost:         "ldap.example.com",
		LDAPPort:         389,
		LDAPBase:         "dc=example,dc=com",
		LDAPBindDN:       "cn=svc,dc=example,dc=com",
		LDAPBindPassword: "old-plaintext",
		WorkflowState:    "active",
	}
	_ = repo.Create(context.Background(), existing)

	svc := service.NewAuthProviderService(repo)
	h := NewAuthProviderHandler(svc)

	app := fiber.New()
	app.Put("/accounts/:account_id/authentication_providers/:id", h.UpdateProvider)

	// Rotation: admin supplies a new bind password.
	body := map[string]any{
		"auth_type":          "ldap",
		"ldap_bind_password": "rotated-secret",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/accounts/1/authentication_providers/1", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status: got %d, want 200; body=%s", resp.StatusCode, respBody)
	}

	if len(repo.updated) != 1 {
		t.Fatalf("expected 1 update, got %d", len(repo.updated))
	}
	stored := repo.updated[len(repo.updated)-1]

	// After rotation the stored row's plaintext field must be blanked
	// and the encrypted column must hold a fresh ciphertext that
	// decrypts to the new value.
	if stored.LDAPBindPassword != "" {
		t.Errorf("rotation left plaintext column populated: %q", stored.LDAPBindPassword)
	}
	if len(stored.LDAPBindPasswordEncrypted) == 0 {
		t.Fatal("rotation did not populate LDAPBindPasswordEncrypted")
	}
	pt, err := auth.Decrypt(stored.LDAPBindPasswordEncrypted)
	if err != nil {
		t.Fatalf("decrypt rotated ciphertext: %v", err)
	}
	if string(pt) != "rotated-secret" {
		t.Errorf("rotation round-trip: got %q, want %q", pt, "rotated-secret")
	}

	// Sub-case: a no-op update (no password supplied) must NOT clobber
	// the existing ciphertext.
	body2 := map[string]any{
		"auth_type": "ldap",
		"ldap_host": "ldap2.example.com",
	}
	body2Bytes, _ := json.Marshal(body2)
	req2 := httptest.NewRequest("PUT", "/accounts/1/authentication_providers/1", bytes.NewReader(body2Bytes))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("second app.Test: %v", err)
	}
	if resp2.StatusCode != fiber.StatusOK {
		t.Fatalf("second update status: %d", resp2.StatusCode)
	}
	stored2 := repo.updated[len(repo.updated)-1]
	if !bytes.Equal(stored2.LDAPBindPasswordEncrypted, stored.LDAPBindPasswordEncrypted) {
		t.Error("no-op update clobbered the existing ciphertext — rotation guard broken")
	}
}

// Avoid an "imported and not used" lint if a future refactor drops
// the os import from the test helper.
var _ = os.Getenv
