package handlers_test

// G136 follow-ups: redeeming a session_url must not replace another
// user's browser session (login CSRF), and a tenant MFA enrollment
// requirement still applies to session-token logins. All data is made up.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestSessionToken_RedeemRefusesOtherUsersSessionAndKeepsMFAEnrollment(t *testing.T) {
	const secret = "test-secret-made-up"
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	owner := &models.User{ID: 650, AccountID: 1, Name: "Olive Owner", LoginID: "olive.o@example.test", Email: "olive.o@example.test", PasswordHash: "x", Role: "user"}
	victim := &models.User{ID: 651, AccountID: 1, Name: "Vic Tim", LoginID: "vic.t@example.test", Email: "vic.t@example.test", PasswordHash: "x", Role: "user"}
	require.NoError(t, users.Create(ctx, owner))
	require.NoError(t, users.Create(ctx, victim))

	pipeline := auth.NewLoginPipeline(users, postgres.NewFederatedIdentityRepository(g),
		postgres.NewAuthenticationProviderRepository(g), postgres.NewAccountRepository(g), auth.NewAuthAudit(nil), secret)
	h := handlers.NewSessionTokenHandler(service.NewLoginSessionTokenService(postgres.NewLoginSessionTokenRepository(g)), pipeline, "https://lms.example.com")
	h.SetSessionSecret(secret)

	app := testutil.SetupTestApp()
	app.Get("/login/session_token", h.Redeem, func(c *fiber.Ctx) error {
		c.Locals("user_id", owner.ID)
		c.Locals("account_id", uint(1))
		c.Locals("access_token_id", uint(77))
		return c.Next()
	}, h.Mint)

	mint := func() string {
		resp := testutil.MakeRequest(app, http.MethodGet, "/login/session_token?return_to=%2Fdashboard", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := testutil.ParseJSONMap(resp)
		u, err := url.Parse(body["session_url"].(string))
		require.NoError(t, err)
		return u.Query().Get("session_token")
	}
	redeem := func(raw, cookie string) *http.Response {
		req := httptest.NewRequest(http.MethodGet, "/login/session_token?session_token="+raw, nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "paper_session", Value: cookie})
		}
		r, err := app.Test(req, -1)
		require.NoError(t, err)
		return r
	}

	// The victim is signed in; the attacker's (owner's) link must not
	// swap their session.
	victimSession, err := auth.GenerateToken(victim, secret)
	require.NoError(t, err)
	r := redeem(mint(), victimSession)
	assert.Equal(t, "https://lms.example.com/login?error=session_token_conflict", r.Header.Get("Location"))
	assert.Empty(t, r.Header.Get("Set-Cookie"))

	// The same user's existing session, or a dead cookie, is fine.
	ownerSession, err := auth.GenerateToken(owner, secret)
	require.NoError(t, err)
	r = redeem(mint(), ownerSession)
	assert.Equal(t, "https://lms.example.com/dashboard", r.Header.Get("Location"))
	r = redeem(mint(), "not-a-jwt")
	assert.Equal(t, "https://lms.example.com/dashboard", r.Header.Get("Location"))
	assert.NotEmpty(t, r.Header.Get("Set-Cookie"))

	// The tenant requires MFA and the owner has not enrolled: the
	// session token still lands on enrollment, as every login does.
	require.NoError(t, g.Model(&models.Account{}).Where("id = ?", 1).Update("mfa_policy", "required_all").Error)
	r = redeem(mint(), "")
	assert.Equal(t, "https://lms.example.com/mfa/enroll", r.Header.Get("Location"))
}
