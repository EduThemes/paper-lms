package handlers_test

// G136 (OAuth2 provider fidelity: PKCE, root paths' error redirects,
// HTTP Basic client auth, RFC 6749 error bodies, token DELETE) and G143
// (personal access token expiry, regenerate, admin management).

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

const pkceVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk-made-up-verifier"

func pkceChallengeFor(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type oauthPKCEFixture struct {
	app       *fiber.App
	tokenRepo *mocks.MockAccessTokenRepository
}

func newOAuthPKCEFixture(t *testing.T, requireScopes bool) *oauthPKCEFixture {
	t.Helper()
	devRepo := new(mocks.MockDeveloperKeyRepository)
	devRepo.On("FindByClientID", mock.Anything, "dk_pkce").Return(&models.DeveloperKey{
		ID: 7, AccountID: 1, Name: "Example Native App", ClientID: "dk_pkce",
		ClientSecret: service.HashClientSecret("pkce-secret"), RedirectURI: "https://app.example.com/cb",
		WorkflowState: "active", RequireScopes: requireScopes, Scopes: `["url:GET|/api/v1/courses"]`,
	}, nil)
	devRepo.On("FindByClientID", mock.Anything, mock.Anything).Return(nil, assert.AnError)
	tokenRepo := new(mocks.MockAccessTokenRepository)
	tokenRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	devKeySvc := service.NewDeveloperKeyService(devRepo)
	tokenSvc := service.NewAccessTokenService(tokenRepo)
	h := handlers.NewOAuth2Handler(service.NewOAuth2Service(devKeySvc, tokenSvc), devKeySvc, tokenSvc, nil)
	h.SetFrontendURL("https://lms.example.com")

	app := testutil.SetupTestApp()
	app.Get("/login/oauth2/auth", h.Authorize)
	app.Post("/login/oauth2/token", h.Token)
	app.Post("/consent", integrationsCallerStub(10, 1, ""), h.AuthorizePost)
	app.Delete("/login/oauth2/token", func(c *fiber.Ctx) error {
		if c.Get("X-Test-Token") != "" {
			c.Locals("access_token_id", uint(55))
		}
		c.Locals("user_id", uint(10))
		return c.Next()
	}, h.Logout)
	return &oauthPKCEFixture{app: app, tokenRepo: tokenRepo}
}

func (f *oauthPKCEFixture) consent(t *testing.T, extra map[string]interface{}) string {
	t.Helper()
	body := map[string]interface{}{"client_id": "dk_pkce", "redirect_uri": "https://app.example.com/cb", "approve": true}
	for k, v := range extra {
		body[k] = v
	}
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/consent", testutil.JSONBody(body))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	out, _ := testutil.ParseJSONMap(resp)
	u, err := url.Parse(out["redirect_url"].(string))
	require.NoError(t, err)
	code := u.Query().Get("code")
	require.NotEmpty(t, code)
	return code
}

func (f *oauthPKCEFixture) tokenForm(t *testing.T, form url.Values, basicUser, basicPass string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/login/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if basicUser != "" {
		req.SetBasicAuth(basicUser, basicPass)
	}
	resp, err := f.app.Test(req, -1)
	require.NoError(t, err)
	body, _ := testutil.ParseJSONMap(resp)
	return resp.StatusCode, body
}

func TestOAuth2Authorize_ErrorsGoBackToTheClient(t *testing.T) {
	f := newOAuthPKCEFixture(t, true)
	base := "/login/oauth2/auth?client_id=dk_pkce&redirect_uri=" + url.QueryEscape("https://app.example.com/cb") + "&state=s9"

	loc := func(path string) *url.URL {
		resp := testutil.MakeRequest(f.app, http.MethodGet, path, nil)
		require.Equal(t, http.StatusFound, resp.StatusCode, path)
		u, err := url.Parse(resp.Header.Get("Location"))
		require.NoError(t, err)
		return u
	}

	u := loc(base + "&response_type=token&scope=" + url.QueryEscape("url:GET|/api/v1/courses"))
	assert.Equal(t, "app.example.com", u.Host)
	assert.Equal(t, "unsupported_response_type", u.Query().Get("error"))
	assert.Equal(t, "s9", u.Query().Get("state"))

	u = loc(base + "&response_type=code")
	assert.Equal(t, "invalid_scope", u.Query().Get("error"), "a require_scopes key must name scopes")
	u = loc(base + "&response_type=code&scope=" + url.QueryEscape("url:DELETE|/api/v1/courses"))
	assert.Equal(t, "invalid_scope", u.Query().Get("error"))

	u = loc(base + "&response_type=code&scope=" + url.QueryEscape("url:GET|/api/v1/courses") + "&code_challenge=" + pkceChallengeFor(pkceVerifier) + "&code_challenge_method=plain")
	assert.Equal(t, "invalid_request", u.Query().Get("error"), "only S256 is accepted")

	// prompt=none can never be satisfied silently (consent is always
	// shown), so the client hears so on its redirect_uri.
	u = loc(base + "&response_type=code&scope=" + url.QueryEscape("url:GET|/api/v1/courses") + "&prompt=none")
	assert.Equal(t, "app.example.com", u.Host)
	assert.Equal(t, "interaction_required", u.Query().Get("error"))
	assert.Equal(t, "s9", u.Query().Get("state"))

	// A valid request goes to the consent screen with the PKCE challenge.
	u = loc(base + "&response_type=code&scopes=" + url.QueryEscape("url:GET|/api/v1/courses") + "&code_challenge=" + pkceChallengeFor(pkceVerifier) + "&code_challenge_method=S256")
	assert.Equal(t, "lms.example.com", u.Host)
	assert.Equal(t, "/login/oauth2/confirm", u.Path)
	assert.Equal(t, pkceChallengeFor(pkceVerifier), u.Query().Get("code_challenge"))
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"))
	assert.Equal(t, "url:GET|/api/v1/courses", u.Query().Get("scope"))
}

func TestOAuth2Token_PKCE(t *testing.T) {
	f := newOAuthPKCEFixture(t, false)
	challenge := pkceChallengeFor(pkceVerifier)

	// A public client: code_verifier and no client_secret.
	code := f.consent(t, map[string]interface{}{"code_challenge": challenge, "code_challenge_method": "S256"})
	status, body := f.tokenForm(t, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "client_id": {"dk_pkce"},
		"redirect_uri": {"https://app.example.com/cb"}, "code_verifier": {pkceVerifier},
	}, "", "")
	require.Equal(t, http.StatusOK, status, body)
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])

	// The code is single-use.
	status, body = f.tokenForm(t, url.Values{
		"code": {code}, "client_id": {"dk_pkce"}, "redirect_uri": {"https://app.example.com/cb"}, "code_verifier": {pkceVerifier},
	}, "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"])

	// A wrong verifier is refused even with the right secret.
	code = f.consent(t, map[string]interface{}{"code_challenge": challenge, "code_challenge_method": "S256"})
	status, body = f.tokenForm(t, url.Values{
		"code": {code}, "client_id": {"dk_pkce"}, "client_secret": {"pkce-secret"},
		"redirect_uri": {"https://app.example.com/cb"}, "code_verifier": {strings.Repeat("x", 50)},
	}, "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_grant", body["error"])

	// A PKCE code can't be redeemed without its verifier.
	code = f.consent(t, map[string]interface{}{"code_challenge": challenge, "code_challenge_method": "S256"})
	status, _ = f.tokenForm(t, url.Values{
		"code": {code}, "client_id": {"dk_pkce"}, "client_secret": {"pkce-secret"}, "redirect_uri": {"https://app.example.com/cb"},
	}, "", "")
	assert.Equal(t, http.StatusBadRequest, status)

	// Malformed challenges are refused at consent.
	resp := testutil.MakeRequest(f.app, http.MethodPost, "/consent", testutil.JSONBody(map[string]interface{}{
		"client_id": "dk_pkce", "redirect_uri": "https://app.example.com/cb", "approve": true,
		"code_challenge": "short", "code_challenge_method": "S256",
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOAuth2Token_ClientAuthAndErrors(t *testing.T) {
	f := newOAuthPKCEFixture(t, false)

	// No PKCE: the secret is required, may come over HTTP Basic, and
	// grant_type defaults to authorization_code when a code is sent.
	code := f.consent(t, nil)
	status, body := f.tokenForm(t, url.Values{"code": {code}, "redirect_uri": {"https://app.example.com/cb"}}, "dk_pkce", "pkce-secret")
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, "Bearer", body["token_type"])

	code = f.consent(t, nil)
	status, body = f.tokenForm(t, url.Values{"code": {code}, "client_id": {"dk_pkce"}, "redirect_uri": {"https://app.example.com/cb"}}, "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "invalid_request", body["error"])

	status, body = f.tokenForm(t, url.Values{"code": {code}, "client_id": {"dk_pkce"}, "client_secret": {"wrong"}, "redirect_uri": {"https://app.example.com/cb"}}, "", "")
	assert.Equal(t, http.StatusUnauthorized, status)
	assert.Equal(t, "invalid_client", body["error"])
	assert.NotEmpty(t, body["errors"], "the Paper/Canvas errors array is kept")

	status, body = f.tokenForm(t, url.Values{"grant_type": {"password"}}, "", "")
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "unsupported_grant_type", body["error"])
}

func TestOAuth2Logout_RevokesTheCallingToken(t *testing.T) {
	f := newOAuthPKCEFixture(t, false)
	f.tokenRepo.On("FindByID", mock.Anything, uint(55)).Return(&models.AccessToken{ID: 55, UserID: 10, WorkflowState: "active"}, nil)
	f.tokenRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	// A cookie session (no access token) can't use it.
	resp := testutil.MakeRequest(f.app, http.MethodDelete, "/login/oauth2/token", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	f.tokenRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)

	req := httptest.NewRequest(http.MethodDelete, "/login/oauth2/token?expire_sessions=1", nil)
	req.Header.Set("X-Test-Token", "1")
	req.AddCookie(&http.Cookie{Name: "paper_session", Value: "jwt-made-up"})
	r, err := f.app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, r.StatusCode)
	revoked := f.tokenRepo.Calls[len(f.tokenRepo.Calls)-1].Arguments.Get(1).(*models.AccessToken)
	assert.Equal(t, "deleted", revoked.WorkflowState)
	assert.Contains(t, r.Header.Get("Set-Cookie"), "paper_session=;")
}

func TestScopeAlwaysAllowed_TokenEndpoint(t *testing.T) {
	assert.True(t, service.ScopeAlwaysAllowed("/login/oauth2/token"))
	assert.True(t, service.ScopeAlwaysAllowed("/api/v1/login/oauth2/token"))
	assert.False(t, service.ScopeAlwaysAllowed("/api/v1/courses"))
}

// --- G143: personal access token lifecycle ---------------------------------

func patApp(tokenRepo *mocks.MockAccessTokenRepository, userRepo *mocks.MockUserRepository, userID, accountID uint) *fiber.App {
	h := handlers.NewAccessTokenHandler(service.NewAccessTokenService(tokenRepo))
	if userRepo != nil {
		h.SetUserRepo(userRepo)
	}
	app := testutil.SetupTestApp()
	app.Use(integrationsCallerStub(userID, accountID, ""), middleware.PaginationParams())
	app.Get("/users/:user_id/tokens", h.ListAccessTokens)
	app.Get("/users/:user_id/user_generated_tokens", h.ListUserGeneratedTokens)
	app.Post("/users/:user_id/tokens", h.CreateAccessToken)
	app.Get("/users/:user_id/tokens/:id", h.GetAccessToken)
	app.Put("/users/:user_id/tokens/:id", h.UpdateAccessToken)
	app.Delete("/users/:user_id/tokens/:id", h.DeleteAccessToken)
	return app
}

func TestAccessTokens_CreateWithExpiry(t *testing.T) {
	tokenRepo := new(mocks.MockAccessTokenRepository)
	var stored *models.AccessToken
	tokenRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AccessToken")).
		Run(func(args mock.Arguments) { stored = args.Get(1).(*models.AccessToken) }).Return(nil)
	app := patApp(tokenRepo, nil, 10, 1)

	when := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	resp := testutil.MakeRequest(app, http.MethodPost, "/users/self/tokens", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"purpose": "Grade sync", "expires_at": when.Format(time.RFC3339)},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	require.NotNil(t, stored.ExpiresAt)
	assert.True(t, stored.ExpiresAt.Equal(when))
	assert.NotEmpty(t, body["token"])
	assert.Equal(t, "personal", body["kind"])

	// A date-only expiry is the end of that day.
	resp = testutil.MakeRequest(app, http.MethodPost, "/users/self/tokens", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"purpose": "Day", "expires_at": time.Now().AddDate(0, 0, 5).Format("2006-01-02")},
	}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	for _, bad := range []string{"yesterday", time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)} {
		resp = testutil.MakeRequest(app, http.MethodPost, "/users/self/tokens", testutil.JSONBody(map[string]interface{}{
			"token": map[string]interface{}{"purpose": "Bad", "expires_at": bad},
		}))
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, bad)
	}
}

func TestAccessTokens_RegenerateAndUpdate(t *testing.T) {
	tokenRepo := new(mocks.MockAccessTokenRepository)
	past := time.Now().Add(-time.Hour)
	expired := &models.AccessToken{ID: 31, UserID: 10, Token: "oldhash", TokenHint: "abcd", Purpose: "Old", ExpiresAt: &past, WorkflowState: "active"}
	devKey := uint(3)
	oauth := &models.AccessToken{ID: 32, UserID: 10, DeveloperKeyID: &devKey, WorkflowState: "active"}
	tokenRepo.On("FindByID", mock.Anything, uint(31)).Return(expired, nil)
	tokenRepo.On("FindByID", mock.Anything, uint(32)).Return(oauth, nil)
	tokenRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)
	app := patApp(tokenRepo, nil, 10, 1)

	// The expired token reads as expired.
	resp := testutil.MakeRequest(app, http.MethodGet, "/users/self/tokens/31", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "expired", body["workflow_state"])
	assert.Equal(t, true, body["can_manually_regenerate"])

	// Regenerating an expired token needs a new expiry (Canvas).
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/self/tokens/31", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"regenerate": true},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	future := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/self/tokens/31", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"regenerate": "true", "expires_at": future, "purpose": "Renamed"},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ = testutil.ParseJSONMap(resp)
	raw, _ := body["token"].(string)
	require.Len(t, raw, 64)
	assert.NotEqual(t, "oldhash", expired.Token)
	assert.Equal(t, raw[len(raw)-4:], expired.TokenHint)
	assert.Equal(t, "Renamed", expired.Purpose)
	assert.Equal(t, "active", body["workflow_state"])

	// Clearing the expiry with a null/blank value.
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/self/tokens/31", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"expires_at": ""},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Nil(t, expired.ExpiresAt)

	// OAuth tokens can't be regenerated by hand.
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/self/tokens/32", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"regenerate": true},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAccessTokens_AdminManagesOwnTenantOnly(t *testing.T) {
	tokenRepo := new(mocks.MockAccessTokenRepository)
	tokenRepo.On("ListByUserID", mock.Anything, uint(20), mock.Anything).Return(&repository.PaginatedResult[models.AccessToken]{
		Items: []models.AccessToken{{ID: 41, UserID: 20, Purpose: "Script", TokenHint: "wxyz", WorkflowState: "active"}}, TotalCount: 1, Page: 1, PerPage: 10,
	}, nil)
	tokenRepo.On("FindByID", mock.Anything, uint(41)).Return(&models.AccessToken{ID: 41, UserID: 20, WorkflowState: "active"}, nil)
	tokenRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, uint(99), uint(0)).Return(&models.User{ID: 99, AccountID: 1, Role: "admin"}, nil)
	userRepo.On("FindByID", mock.Anything, uint(20), uint(1)).Return(&models.User{ID: 20, AccountID: 1}, nil)
	userRepo.On("FindByID", mock.Anything, uint(21), uint(1)).Return(nil, assert.AnError) // another tenant's user
	userRepo.On("FindByID", mock.Anything, uint(22), uint(1)).Return(&models.User{ID: 22, AccountID: 1, Role: "super_admin"}, nil)
	app := patApp(tokenRepo, userRepo, 99, 1)

	resp := testutil.MakeRequest(app, http.MethodGet, "/users/20/tokens", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	require.Len(t, arr, 1)
	assert.Equal(t, false, arr[0]["can_manually_regenerate"], "an admin can't regenerate someone else's token")

	resp = testutil.MakeRequest(app, http.MethodGet, "/users/20/user_generated_tokens", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Delete by token_hint, as Canvas allows.
	resp = testutil.MakeRequest(app, http.MethodDelete, "/users/20/tokens/wxyz", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Admins never mint or change another user's token.
	resp = testutil.MakeRequest(app, http.MethodPost, "/users/20/tokens", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"purpose": "Backdoor"},
	}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/20/tokens/41", testutil.JSONBody(map[string]interface{}{
		"token": map[string]interface{}{"regenerate": true},
	}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	tokenRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	// Another tenant's user is a 404.
	resp = testutil.MakeRequest(app, http.MethodGet, "/users/21/tokens", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// A platform operator homed in this account is out of the account
	// admin's reach too: no listing, no revoking.
	resp = testutil.MakeRequest(app, http.MethodGet, "/users/22/tokens", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodDelete, "/users/22/tokens/1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	tokenRepo.AssertNotCalled(t, "ListByUserID", mock.Anything, uint(22), mock.Anything)
}
