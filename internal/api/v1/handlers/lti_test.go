package handlers_test

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// fakeSessionMiddleware seeds Locals("user_id") so the LTI handlers'
// requireSessionUserMatches gate sees an authenticated caller. When
// userID is 0, the gate falls through to 401 — that's how the
// "anonymous caller" repro is exercised.
func fakeSessionMiddleware(userID uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if userID > 0 {
			c.Locals("user_id", userID)
		}
		return c.Next()
	}
}

// setupLTIHandlerForGate wires an LTIHandler with only the gate-relevant
// repos populated. The service slots stay nil; the gate runs before
// any service call, so the BadRequest fallback on the empty
// lti_message_hint is the success signal when the gate allows the
// request through. sessionUserID is the user the test wishes to "be"
// (set as Locals before the handler).
func setupLTIHandlerForGate(sessionUserID uint) (
	*fiber.App,
	*mocks.MockUserRepository,
	*mocks.MockAccountRepository,
	*mocks.MockParentalConsentRepository,
) {
	userRepo := new(mocks.MockUserRepository)
	accountRepo := new(mocks.MockAccountRepository)
	consentRepo := new(mocks.MockParentalConsentRepository)

	h := handlers.NewLTIHandler(nil, nil, nil, nil, nil, userRepo, accountRepo, consentRepo)

	app := testutil.SetupTestApp()
	// PENTEST F-055 review follow-up: production routes `/lti/launch` to
	// LaunchDirect (router.go), not Launch. Tests now exercise the same
	// handler production runs, so the gate coverage is genuinely
	// production-equivalent. The two methods share `requireSessionUserMatches`
	// and `gateLTILaunchForCOPPA`, so the gate semantics are identical.
	app.Post("/lti/launch", fakeSessionMiddleware(sessionUserID), h.LaunchDirect)
	return app, userRepo, accountRepo, consentRepo
}

// jsonLaunchBody returns a JSON body for a LaunchDirect call. The
// message hint is deliberately invalid ("not-a-number:..."): the gate
// (F-055 + COPPA) runs BEFORE parseExtendedMessageHint, so a parse
// failure becomes 400 which gate tests treat as non-403/non-401 success.
// Using a valid hint would reach the configRepo dependency (nil here),
// turning gate-allow paths into nil-pointer panics.
func jsonLaunchBody(loginHint string) interface{} {
	return map[string]interface{}{
		"login_hint":       loginHint,
		"lti_message_hint": "not-a-course-id:resource-link-1:1",
	}
}

// TestLTIOIDCLogin_HigherEdAllows — the gate is bypassed for non-COPPA
// tenants. The handler then drops to its service call (nil here), which
// would panic or 500 — but the gate test only needs NOT-403 to pass.
func TestLTIOIDCLogin_HigherEdAllows(t *testing.T) {
	app, userRepo, accountRepo, _ := setupLTIHandlerForGate(7)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: false,
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	// Gate let it through; downstream nil-service path produces a non-403.
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_K5RefusesWithoutConsent — k5 tenant, no granted
// third_party_sharing consent = 403.
func TestLTIOIDCLogin_K5RefusesWithoutConsent(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate(7)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_CoppaStrictRefuses — higher_ed tenant_mode but
// CoppaStrict=true = 403.
func TestLTIOIDCLogin_CoppaStrictRefuses(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate(7)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: true,
	}, nil)
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// TestLTIOIDCLogin_K5AllowsWithGrantedConsent — k5 tenant + granted
// third_party_sharing consent = bypass the 403.
func TestLTIOIDCLogin_K5AllowsWithGrantedConsent(t *testing.T) {
	app, userRepo, accountRepo, consentRepo := setupLTIHandlerForGate(7)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("k5"),
		CoppaStrict: false,
	}, nil)
	now := time.Now()
	consentRepo.On("FindByStudentID", mock.Anything, uint(7)).Return([]models.ParentalConsent{
		{StudentID: 7, ConsentType: "third_party_sharing", Status: "granted", ConsentedAt: &now},
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.NotEqual(t, http.StatusForbidden, resp.StatusCode)
}

// --------------------------------------------------------------------------
// F-055: LTI launch must reject anonymous callers and login_hint
// mismatches. The pre-fix path trusted login_hint from the request body
// and minted an id_token impersonating any user. The fix is the
// requireSessionUserMatches helper that pins identity to the session
// JWT (Locals("user_id")) and ignores attacker-controlled hints.
// --------------------------------------------------------------------------

// TestLTILaunch_RejectsAnonymousCaller — sessionUserID=0 (no session
// middleware fires) MUST yield 401 regardless of what login_hint says.
// This is the core F-055 repro: an unauthenticated caller forging
// login_hint=<victim> previously got a signed launch token.
func TestLTILaunch_RejectsAnonymousCaller(t *testing.T) {
	app, _, _, _ := setupLTIHandlerForGate(0)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("7")))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestLTILaunch_RejectsLoginHintMismatch — session is user 7, but the
// request claims login_hint=42. The handler MUST 401 rather than mint a
// token for user 42.
func TestLTILaunch_RejectsLoginHintMismatch(t *testing.T) {
	app, _, _, _ := setupLTIHandlerForGate(7)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(jsonLaunchBody("42")))
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestLTILaunch_RejectsNonNumericLoginHint — login_hint must parse as a
// decimal user ID when provided; gibberish 400s.
func TestLTILaunch_RejectsNonNumericLoginHint(t *testing.T) {
	app, _, _, _ := setupLTIHandlerForGate(7)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(map[string]interface{}{
		"client_id":  "abc",
		"login_hint": "not-a-number",
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestLTILaunch_AllowsEmptyHintWhenSessionPresent — when login_hint is
// omitted, the session user IS the launch user. This is the legitimate
// "platform initiates from logged-in browser" path. The gate then
// continues into COPPA / downstream service; nil services bail with a
// non-401 response (verifying the gate let us past 401).
func TestLTILaunch_AllowsEmptyHintWhenSessionPresent(t *testing.T) {
	app, userRepo, accountRepo, _ := setupLTIHandlerForGate(7)

	userRepo.On("FindByID", mock.Anything, uint(7), uint(0)).Return(&models.User{ID: 7, AccountID: 1}, nil)
	accountRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.Account{
		ID:          1,
		TenantMode:  models.GamificationAudience("higher_ed"),
		CoppaStrict: false,
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodPost, "/lti/launch", testutil.JSONBody(map[string]interface{}{
		"client_id": "abc",
	}))
	assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
}

// --------------------------------------------------------------------------
// F-056: redirect_uri in the launch HTML must be escaped (or, as the
// fix does, sourced ONLY from the tool config's TargetLinkURI). The
// pre-fix path Sprintf'd attacker-controlled redirect_uri into the
// action= attribute. We can't easily render the full launch HTML here
// without wiring the full service stack, so we use a direct
// html/template test to lock in the contract: an attacker payload in
// the RedirectURI field comes out HTML-escaped and cannot break out
// of the attribute.
// --------------------------------------------------------------------------

// TestLTILaunchTemplate_EscapesRedirectURI — feed the canonical
// "><script>" payload directly into the template and assert it's
// rendered as escaped text, not as live HTML. This locks in the F-056
// contract at the template layer: even if a future code path
// accidentally takes a user value, the template auto-escapes.
func TestLTILaunchTemplate_EscapesRedirectURI(t *testing.T) {
	// We replicate the template inline so the test doesn't depend on
	// exporting the variable. If the template changes shape, this test
	// is the canary.
	out := renderLTILaunchHTML(t, `"><script>alert('xss')</script><a href="`, "tok")
	// The XSS payload's literal `<script>alert` must not appear as
	// live HTML — html/template's contextual auto-escape converts it
	// into URL-encoded characters inside the action= attribute
	// (%3c%73%63%72%69%70%74…) or HTML entities depending on context.
	if strings.Contains(out, "<script>alert") {
		t.Fatalf("expected redirect_uri to be escaped; got live script in output: %s", out)
	}
	// The attacker's quote-and-tag break-out must NOT close the action
	// attribute and start a new tag. Confirm the only `<script>` left
	// in the rendered HTML is the legitimate inline submit script at
	// the end of the template.
	scriptCount := strings.Count(out, "<script>")
	if scriptCount != 1 {
		t.Fatalf("expected exactly one <script> tag (the auto-submit one); got %d in: %s", scriptCount, out)
	}
}

// renderLTILaunchHTML is a test helper that drives the real launch
// flow via Launch() and reads back the response body. We mock the gate
// repos so the COPPA path is a no-op, then we have to bail before the
// service call — but the redirect_uri value is what we want to assert
// on, and the handler bails on missing tool config (BadRequest) before
// reaching the template render. So we instead use a tiny inline
// template that mirrors the production one. The string match in the
// caller is what enforces the escape contract.
func renderLTILaunchHTML(t *testing.T, redirectURI, idToken string) string {
	t.Helper()
	app := testutil.SetupTestApp()
	app.Get("/render", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/html; charset=utf-8")
		return handlers.RenderLTILaunchForTest(c.Response().BodyWriter(), redirectURI, idToken)
	})
	resp := testutil.MakeRequest(app, http.MethodGet, "/render", nil)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return string(body)
}
