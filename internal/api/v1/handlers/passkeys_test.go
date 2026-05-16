// Handler-layer tests for the WebAuthn (passkey) routes.
//
// Sprint 10-B shipped without handler tests — only `internal/auth/webauthn_test.go`
// covered session encoding + pipeline outcome shape. These tests close
// the regression gap on the HTTP surface: every route exercised, every
// auth-gate verified, every cookie-driven failure mode pinned.
//
// What is NOT tested here: the full FinishRegistration / FinishLogin
// flow. Those require a real browser WebAuthn assertion (which can't
// be synthesized in Go without a virtual authenticator). Their cookie
// failure modes (no ceremony cookie → 400) ARE tested; the
// happy-path is exercised end-to-end by the `internal/auth/webauthn_test.go`
// session round-trip + the LoginPipeline matrix.
package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// ----- happy + sad: List -----

func TestPasskeyHandler_List_Empty(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Get("/users/self/passkeys", withUserID(42), h.List)

	resp := testGET(t, app, "/users/self/passkeys")
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Passkeys []map[string]any `json:"passkeys"`
	}
	mustDecode(t, resp, &body)
	if len(body.Passkeys) != 0 {
		t.Errorf("want empty list, got %d items", len(body.Passkeys))
	}
}

func TestPasskeyHandler_List_WithRows(t *testing.T) {
	h, app, _, creds := newPasskeyTestApp(t)
	creds.byUser[42] = []models.UserWebauthnCredential{
		{ID: 1, UserID: 42, Nickname: "MacBook", BackupState: true, CredentialID: []byte("a")},
		{ID: 2, UserID: 42, Nickname: "iPhone", BackupState: false, CredentialID: []byte("b")},
	}
	app.Get("/users/self/passkeys", withUserID(42), h.List)

	resp := testGET(t, app, "/users/self/passkeys")
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Passkeys []struct {
			ID          uint   `json:"id"`
			Nickname    string `json:"nickname"`
			BackupState bool   `json:"backup_state"`
		} `json:"passkeys"`
	}
	mustDecode(t, resp, &body)
	if len(body.Passkeys) != 2 {
		t.Fatalf("want 2 items, got %d", len(body.Passkeys))
	}
	if body.Passkeys[0].Nickname != "MacBook" || !body.Passkeys[0].BackupState {
		t.Errorf("first row shape wrong: %+v", body.Passkeys[0])
	}
}

func TestPasskeyHandler_List_Unauthenticated(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Get("/users/self/passkeys", h.List) // no withUserID middleware

	resp := testGET(t, app, "/users/self/passkeys")
	if resp.StatusCode != 401 {
		t.Fatalf("status: want 401, got %d", resp.StatusCode)
	}
}

// ----- Rename -----

func TestPasskeyHandler_Rename_Happy(t *testing.T) {
	h, app, _, creds := newPasskeyTestApp(t)
	creds.byUser[42] = []models.UserWebauthnCredential{{ID: 7, UserID: 42, Nickname: "old"}}
	app.Patch("/users/self/passkeys/:id", withUserID(42), h.Rename)

	resp := testJSON(t, app, "PATCH", "/users/self/passkeys/7", map[string]string{"nickname": "new"})
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if got := creds.byUser[42][0].Nickname; got != "new" {
		t.Errorf("nickname not persisted: got %q", got)
	}
}

func TestPasskeyHandler_Rename_NotFound(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Patch("/users/self/passkeys/:id", withUserID(42), h.Rename)

	resp := testJSON(t, app, "PATCH", "/users/self/passkeys/9999", map[string]string{"nickname": "x"})
	if resp.StatusCode != 404 {
		t.Fatalf("status: want 404, got %d", resp.StatusCode)
	}
}

func TestPasskeyHandler_Rename_BadID(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Patch("/users/self/passkeys/:id", withUserID(42), h.Rename)

	resp := testJSON(t, app, "PATCH", "/users/self/passkeys/notanint", map[string]string{"nickname": "x"})
	if resp.StatusCode != 400 {
		t.Fatalf("status: want 400, got %d", resp.StatusCode)
	}
}

func TestPasskeyHandler_Rename_Unauthenticated(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Patch("/users/self/passkeys/:id", h.Rename)

	resp := testJSON(t, app, "PATCH", "/users/self/passkeys/1", map[string]string{"nickname": "x"})
	if resp.StatusCode != 401 {
		t.Fatalf("status: want 401, got %d", resp.StatusCode)
	}
}

// ----- Revoke -----

func TestPasskeyHandler_Revoke_Happy(t *testing.T) {
	h, app, _, creds := newPasskeyTestApp(t)
	creds.byUser[42] = []models.UserWebauthnCredential{{ID: 7, UserID: 42}}
	app.Delete("/users/self/passkeys/:id", withUserID(42), h.Revoke)

	resp := testDELETE(t, app, "/users/self/passkeys/7")
	if resp.StatusCode != 204 {
		t.Fatalf("status: want 204, got %d", resp.StatusCode)
	}
	if len(creds.byUser[42]) != 0 {
		t.Errorf("expected row removed; still have %d", len(creds.byUser[42]))
	}
}

func TestPasskeyHandler_Revoke_NotFound(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Delete("/users/self/passkeys/:id", withUserID(42), h.Revoke)

	resp := testDELETE(t, app, "/users/self/passkeys/999")
	if resp.StatusCode != 404 {
		t.Fatalf("status: want 404, got %d", resp.StatusCode)
	}
}

func TestPasskeyHandler_Revoke_CrossUserScoped(t *testing.T) {
	// Per repo doc-comment: Delete is scoped to (id, user_id). Caller
	// 42 cannot delete user 7's row even if they know the id.
	h, app, _, creds := newPasskeyTestApp(t)
	creds.byUser[7] = []models.UserWebauthnCredential{{ID: 100, UserID: 7}}
	app.Delete("/users/self/passkeys/:id", withUserID(42), h.Revoke)

	resp := testDELETE(t, app, "/users/self/passkeys/100")
	if resp.StatusCode != 404 {
		t.Fatalf("status: want 404 (scoped), got %d", resp.StatusCode)
	}
	if len(creds.byUser[7]) != 1 {
		t.Errorf("user 7's row was deleted by an unauthorized caller")
	}
}

// ----- BeginRegistration -----

func TestPasskeyHandler_BeginRegistration_SetsCeremonyCookie(t *testing.T) {
	h, app, users, _ := newPasskeyTestApp(t)
	users.byID[42] = &models.User{
		ID:                 42,
		LoginID:            "alice@test",
		Email:              "alice@test",
		Name:               "Alice",
		WebauthnUserHandle: randomBytes(t, 64),
	}
	app.Post("/users/self/passkeys/begin", withUserID(42), h.BeginRegistration)

	resp := testJSON(t, app, "POST", "/users/self/passkeys/begin", map[string]string{"nickname": "MacBook"})
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	var foundCeremony, foundNick bool
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "passkey_ceremony" && cookie.Value != "" {
			foundCeremony = true
		}
		if cookie.Name == "passkey_ceremony_nick" && cookie.Value == "MacBook" {
			foundNick = true
		}
	}
	if !foundCeremony {
		t.Errorf("ceremony cookie not set")
	}
	if !foundNick {
		t.Errorf("nickname cookie not set")
	}
}

func TestPasskeyHandler_BeginRegistration_Unauthenticated(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Post("/users/self/passkeys/begin", h.BeginRegistration)
	resp := testJSON(t, app, "POST", "/users/self/passkeys/begin", map[string]string{})
	if resp.StatusCode != 401 {
		t.Fatalf("status: want 401, got %d", resp.StatusCode)
	}
}

func TestPasskeyHandler_BeginRegistration_UserNotFound(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Post("/users/self/passkeys/begin", withUserID(42), h.BeginRegistration)
	resp := testJSON(t, app, "POST", "/users/self/passkeys/begin", map[string]string{})
	if resp.StatusCode != 404 {
		t.Fatalf("status: want 404, got %d", resp.StatusCode)
	}
}

// ----- BeginLogin -----

func TestPasskeyHandler_BeginLogin_SetsCeremonyCookie(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Post("/auth/passkey/begin", h.BeginLogin)

	resp := testJSON(t, app, "POST", "/auth/passkey/begin", map[string]string{})
	if resp.StatusCode != 200 {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	var found bool
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "passkey_ceremony" && cookie.Value != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("ceremony cookie not set on discoverable login begin")
	}
}

// ----- FinishRegistration / FinishLogin without cookie -----

func TestPasskeyHandler_FinishRegistration_NoCeremonyCookie_400(t *testing.T) {
	h, app, users, _ := newPasskeyTestApp(t)
	users.byID[42] = &models.User{ID: 42, LoginID: "x", Email: "x"}
	app.Post("/users/self/passkeys/finish", withUserID(42), h.FinishRegistration)

	resp := testJSON(t, app, "POST", "/users/self/passkeys/finish", map[string]string{})
	if resp.StatusCode != 400 {
		t.Fatalf("status: want 400 (expired ceremony), got %d", resp.StatusCode)
	}
}

func TestPasskeyHandler_FinishLogin_NoCeremonyCookie_400(t *testing.T) {
	h, app, _, _ := newPasskeyTestApp(t)
	app.Post("/auth/passkey/finish", h.FinishLogin)

	resp := testJSON(t, app, "POST", "/auth/passkey/finish", map[string]string{})
	if resp.StatusCode != 400 {
		t.Fatalf("status: want 400 (expired ceremony), got %d", resp.StatusCode)
	}
}

// ----- helpers -----

func newPasskeyTestApp(t *testing.T) (*handlers.PasskeyHandler, *fiber.App, *fakePasskeyUserRepo, *fakeCredsRepo) {
	t.Helper()
	// secretbox needs an encryption key for cookie encoding.
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("gen key: %v", err)
	}
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
	if err := auth.EnsureKeysLoaded(); err != nil {
		t.Fatalf("load keys: %v", err)
	}

	users := newFakeUserRepo()
	creds := newFakeCredsRepo()
	engine, err := auth.NewPasskeyEngine("Paper LMS Test", "localhost", []string{"http://localhost:3000"}, users, creds)
	if err != nil {
		t.Fatalf("engine: %v", err)
	}
	audit := &auth.AuthAudit{} // nil-svc — emit is a defensive no-op
	h := handlers.NewPasskeyHandler(engine, users, creds, nil, audit, "test-jwt-secret")

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"errors": []fiber.Map{{"message": err.Error()}},
			})
		},
	})
	return h, app, users, creds
}

func withUserID(uid uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uid)
		c.Locals("account_id", uint(1))
		return c.Next()
	}
}

func testGET(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func testDELETE(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest("DELETE", path, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

func testJSON(t *testing.T, app *fiber.App, method, path string, payload any) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func readAll(t *testing.T, r io.ReadCloser) []byte {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return b
}

func mustDecode(t *testing.T, resp *http.Response, into any) {
	t.Helper()
	b := readAll(t, resp.Body)
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, b)
	}
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return b
}

// ----- fake repos -----

type fakePasskeyUserRepo struct {
	byID map[uint]*models.User
}

func newFakeUserRepo() *fakePasskeyUserRepo {
	return &fakePasskeyUserRepo{byID: map[uint]*models.User{}}
}

func (r *fakePasskeyUserRepo) FindByID(_ context.Context, id uint) (*models.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (r *fakePasskeyUserRepo) Create(context.Context, *models.User) error { panic("unused") }
func (r *fakePasskeyUserRepo) FindByLoginID(context.Context, string) (*models.User, error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) FindByEmail(context.Context, string) (*models.User, error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) FindBySISUserID(context.Context, string) (*models.User, error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) FindByIDs(context.Context, []uint) ([]models.User, error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) Update(context.Context, *models.User) error { panic("unused") }
func (r *fakePasskeyUserRepo) List(context.Context, repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) FindByResetToken(context.Context, string) (*models.User, error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) Search(context.Context, string, repository.PaginationParams) (*repository.PaginatedResult[models.User], error) {
	panic("unused")
}
func (r *fakePasskeyUserRepo) FilterPublicLeaderboardCandidates(context.Context, []uint) ([]uint, error) {
	panic("unused")
}

type fakeCredsRepo struct {
	byUser map[uint][]models.UserWebauthnCredential
}

func newFakeCredsRepo() *fakeCredsRepo {
	return &fakeCredsRepo{byUser: map[uint][]models.UserWebauthnCredential{}}
}

func (r *fakeCredsRepo) Create(_ context.Context, c *models.UserWebauthnCredential) error {
	r.byUser[c.UserID] = append(r.byUser[c.UserID], *c)
	return nil
}

func (r *fakeCredsRepo) FindByCredentialID(_ context.Context, credentialID []byte) (*models.UserWebauthnCredential, error) {
	for _, list := range r.byUser {
		for _, c := range list {
			if bytes.Equal(c.CredentialID, credentialID) {
				cc := c
				return &cc, nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeCredsRepo) FindByID(_ context.Context, id uint) (*models.UserWebauthnCredential, error) {
	for _, list := range r.byUser {
		for _, c := range list {
			if c.ID == id {
				cc := c
				return &cc, nil
			}
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeCredsRepo) ListForUser(_ context.Context, userID uint) ([]models.UserWebauthnCredential, error) {
	return append([]models.UserWebauthnCredential{}, r.byUser[userID]...), nil
}

func (r *fakeCredsRepo) UpdateSignCount(_ context.Context, id uint, newSignCount uint32) error {
	for uid, list := range r.byUser {
		for i := range list {
			if list[i].ID == id {
				r.byUser[uid][i].SignCount = newSignCount
				return nil
			}
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeCredsRepo) UpdateNickname(_ context.Context, id, userID uint, nickname string) error {
	list := r.byUser[userID]
	for i := range list {
		if list[i].ID == id {
			r.byUser[userID][i].Nickname = nickname
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

func (r *fakeCredsRepo) Delete(_ context.Context, id, userID uint) error {
	list := r.byUser[userID]
	for i := range list {
		if list[i].ID == id {
			r.byUser[userID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return gorm.ErrRecordNotFound
}

// Compile-time interface satisfaction guards. If a method is added to
// either interface, these fakes need to be extended too.
var _ repository.UserRepository = (*fakePasskeyUserRepo)(nil)
var _ repository.UserWebauthnCredentialRepository = (*fakeCredsRepo)(nil)
