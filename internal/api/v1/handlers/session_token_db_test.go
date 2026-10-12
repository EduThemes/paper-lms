package handlers_test

// G136: GET /login/session_token. An API caller mints a single-use
// session_url; opening it signs the user into a browser session through
// the LoginPipeline, once. DB-backed; all data is made up.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestSessionToken_MintRedeemOnce(t *testing.T) {
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	require.NoError(t, users.Create(ctx, &models.User{ID: 640, AccountID: 1, Name: "Pat Token", LoginID: "pat.token@example.test", Email: "pat.token@example.test", PasswordHash: "x", Role: "user"}))
	suspended := &models.User{ID: 641, AccountID: 1, Name: "Sue Spended", LoginID: "sue.s@example.test", Email: "sue.s@example.test", PasswordHash: "x", Role: "user"}
	require.NoError(t, users.Create(ctx, suspended))

	pipeline := auth.NewLoginPipeline(users, postgres.NewFederatedIdentityRepository(g),
		postgres.NewAuthenticationProviderRepository(g), postgres.NewAccountRepository(g), auth.NewAuthAudit(nil), "test-secret-made-up")
	tokens := service.NewLoginSessionTokenService(postgres.NewLoginSessionTokenRepository(g))
	h := handlers.NewSessionTokenHandler(tokens, pipeline, "https://lms.example.com")

	caller := func(userID uint, viaToken bool) fiber.Handler {
		return func(c *fiber.Ctx) error {
			c.Locals("user_id", userID)
			c.Locals("account_id", uint(1))
			if viaToken {
				c.Locals("access_token_id", uint(77))
			}
			return c.Next()
		}
	}
	app := testutil.SetupTestApp()
	app.Get("/login/session_token", h.Redeem, caller(640, true), h.Mint)
	app.Get("/cookie/login/session_token", caller(640, false), h.Mint)
	app.Get("/suspended/login/session_token", caller(641, true), h.Mint)

	// A cookie session can't mint (Canvas: API only).
	resp := testutil.MakeRequest(app, http.MethodGet, "/cookie/login/session_token", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	// Open redirects are refused.
	resp = testutil.MakeRequest(app, http.MethodGet, "/login/session_token?return_to="+url.QueryEscape("https://evil.example.net/x"), nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodGet, "/login/session_token?return_to="+url.QueryEscape("/courses/12?tab=grades"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	sessionURL, err := url.Parse(body["session_url"].(string))
	require.NoError(t, err)
	assert.Equal(t, "/login/session_token", sessionURL.Path)
	raw := sessionURL.Query().Get("session_token")
	require.Len(t, raw, 64)

	redeem := func() *http.Response {
		r, err := app.Test(httptest.NewRequest(http.MethodGet, "/login/session_token?session_token="+raw, nil), -1)
		require.NoError(t, err)
		return r
	}
	r := redeem()
	require.Equal(t, http.StatusFound, r.StatusCode)
	assert.Equal(t, "https://lms.example.com/courses/12?tab=grades", r.Header.Get("Location"))
	cookie := r.Header.Get("Set-Cookie")
	assert.True(t, strings.HasPrefix(cookie, "paper_session=") && !strings.HasPrefix(cookie, "paper_session=;"), cookie)
	assert.Contains(t, strings.ToLower(cookie), "httponly")

	// Single use.
	r = redeem()
	require.Equal(t, http.StatusFound, r.StatusCode)
	assert.Equal(t, "https://lms.example.com/login?error=session_token", r.Header.Get("Location"))
	assert.Empty(t, r.Header.Get("Set-Cookie"))

	// A user suspended after minting gets no session.
	resp = testutil.MakeRequest(app, http.MethodGet, "/suspended/login/session_token", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ = testutil.ParseJSONMap(resp)
	sessionURL, _ = url.Parse(body["session_url"].(string))
	require.NoError(t, g.Model(&models.User{}).Where("id = ?", 641).Update("suspended", true).Error)
	raw = sessionURL.Query().Get("session_token")
	r = redeem()
	assert.Equal(t, "https://lms.example.com/login?error=session_token", r.Header.Get("Location"))
	assert.Empty(t, r.Header.Get("Set-Cookie"))
}
