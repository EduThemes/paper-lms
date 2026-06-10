package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testJWTSecret = "test-jwt-secret-key"

// testUser creates a simple User model for JWT generation. AccountID is
// set so the 13.1.B account_id claim is populated and the middleware
// doesn't fall back to a userRepo lookup.
func testUser() *models.User {
	return &models.User{
		ID:        42,
		AccountID: 1,
		Email:     "alice@example.com",
		Name:      "Alice Wonderland",
	}
}

// setupProtectedApp creates a Fiber app with the auth middleware and a simple
// handler that returns 200 with the authenticated user_id.
func setupProtectedApp(tokenRepo *mocks.MockAccessTokenRepository, userRepo *mocks.MockUserRepository) *fiber.App {
	app := testutil.SetupTestApp()

	var accessTokenSvc *service.AccessTokenService
	if tokenRepo != nil {
		accessTokenSvc = service.NewAccessTokenService(tokenRepo)
	}

	// Pass a TRUE nil interface (not a typed-nil *MockUserRepository) when
	// no repo is provided, so the middleware's `m.userRepo != nil` guard
	// behaves the same as in production (where the repo is always real).
	var userRepoIface repository.UserRepository
	if userRepo != nil {
		userRepoIface = userRepo
	}
	authMW := middleware.NewAuthMiddleware(testJWTSecret, accessTokenSvc, userRepoIface, nil)

	app.Get("/protected", authMW.Protected(), func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"user_id": c.Locals("user_id"),
			"email":   c.Locals("user_email"),
			"name":    c.Locals("user_name"),
		})
	})

	return app
}

// sha256Hex computes the SHA-256 hex digest of a string.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestProtected_ValidJWTHeader(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	token, err := auth.GenerateToken(testUser(), testJWTSecret)
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", token, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(42), body["user_id"])
	assert.Equal(t, "alice@example.com", body["email"])
	assert.Equal(t, "Alice Wonderland", body["name"])
}

func TestProtected_ValidJWTCookie(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	token, err := auth.GenerateToken(testUser(), testJWTSecret)
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedCookieRequest(app, http.MethodGet, "/protected", token, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(42), body["user_id"])
}

// A suspended user's still-valid JWT must be rejected on EVERY request —
// the kill-switch that makes suspension take effect immediately, not just
// at next login.
func TestProtected_SuspendedUser_JWT_Returns401(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, uint(42), uint(0)).
		Return(&models.User{ID: 42, AccountID: 1, Suspended: true}, nil)
	app := setupProtectedApp(nil, userRepo)

	token, err := auth.GenerateToken(testUser(), testJWTSecret)
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", token, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// Control: a found-but-NOT-suspended user is allowed through — the
// per-request check doesn't over-block when the user repo is present.
func TestProtected_ActiveUser_JWT_Allowed(t *testing.T) {
	userRepo := new(mocks.MockUserRepository)
	userRepo.On("FindByID", mock.Anything, uint(42), uint(0)).
		Return(&models.User{ID: 42, AccountID: 1, Suspended: false}, nil)
	app := setupProtectedApp(nil, userRepo)

	token, err := auth.GenerateToken(testUser(), testJWTSecret)
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", token, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProtected_NoToken(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/protected", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)

	errs, ok := body["errors"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, errs, 1)
	errMap := errs[0].(map[string]interface{})
	assert.Contains(t, errMap["message"], "no token provided")
}

func TestProtected_InvalidJWT(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", "not-a-real-jwt", nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)

	errs := body["errors"].([]interface{})
	errMap := errs[0].(map[string]interface{})
	assert.Contains(t, errMap["message"], "Invalid or expired token")
}

// TestProtected_WrongAudience confirms a structurally valid token
// signed with the right secret but minted for a different audience
// is rejected. This is the cross-service replay defense added by
// audit finding #5 (2026-05-22).
func TestProtected_WrongAudience(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	claims := jwt.MapClaims{
		"id":    float64(42),
		"email": "alice@example.com",
		"name":  "Alice Wonderland",
		"iss":   auth.JWTIssuer,
		"aud":   "some-other-service",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", tokenStr, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestProtected_WrongIssuer mirrors WrongAudience for the iss claim.
// Both must match — a token with the right aud but wrong iss is
// equally rejected.
func TestProtected_WrongIssuer(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	claims := jwt.MapClaims{
		"id":    float64(42),
		"email": "alice@example.com",
		"iss":   "not-paper-lms",
		"aud":   auth.JWTAudienceAPI,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", tokenStr, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestProtected_LegacyJWTWithoutAudience confirms tokens minted before
// the iss/aud enforcement (no iss, no aud) are also rejected. This is
// the user-impact note in the PR: existing sessions get logged out on
// deploy and must re-authenticate. Tradeoff is acceptable given the
// 24h TTL and that the alternative is a downgrade attack surface.
func TestProtected_LegacyJWTWithoutAudience(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	claims := jwt.MapClaims{
		"id":    float64(42),
		"email": "alice@example.com",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", tokenStr, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestProtected_ExpiredJWT(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	// Manually create a JWT that expired 2 hours ago
	claims := jwt.MapClaims{
		"id":    float64(42),
		"email": "alice@example.com",
		"name":  "Alice Wonderland",
		"exp":   time.Now().Add(-2 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(testJWTSecret))
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", tokenStr, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)

	errs := body["errors"].([]interface{})
	errMap := errs[0].(map[string]interface{})
	assert.Contains(t, errMap["message"], "Invalid or expired token")
}

func TestProtected_ValidAccessToken(t *testing.T) {
	mockTokenRepo := new(mocks.MockAccessTokenRepository)
	mockUserRepo := new(mocks.MockUserRepository)
	app := setupProtectedApp(mockTokenRepo, mockUserRepo)

	rawToken := "pat_abc123def456abc123def456abc123def456abc123def456abc123def456ab"
	tokenHash := sha256Hex(rawToken)

	storedToken := &models.AccessToken{
		ID:            10,
		UserID:        99,
		Token:         tokenHash,
		WorkflowState: "active",
	}

	storedUser := &models.User{
		ID:    99,
		Email: "bob@example.com",
		Name:  "Bob Builder",
	}

	// ValidateToken will hash the raw token and call FindByToken
	mockTokenRepo.On("FindByToken", mock.Anything, tokenHash).Return(storedToken, nil)
	mockTokenRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.AccessToken")).Return(nil)

	// Middleware looks up user after successful token validation
	mockUserRepo.On("FindByID", mock.Anything, uint(99), uint(0)).Return(storedUser, nil)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/protected", rawToken, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)
	assert.Equal(t, float64(99), body["user_id"])
	assert.Equal(t, "bob@example.com", body["email"])
	assert.Equal(t, "Bob Builder", body["name"])
	mockTokenRepo.AssertExpectations(t)
	mockUserRepo.AssertExpectations(t)
}

func TestProtected_HeaderPriority(t *testing.T) {
	app := setupProtectedApp(nil, nil)

	// Generate a valid JWT for the header
	headerToken, err := auth.GenerateToken(testUser(), testJWTSecret)
	assert.NoError(t, err)

	// Generate a different valid JWT for the cookie (different user)
	cookieUser := &models.User{
		ID:    99,
		Email: "bob@example.com",
		Name:  "Bob Builder",
	}
	cookieToken, err := auth.GenerateToken(cookieUser, testJWTSecret)
	assert.NoError(t, err)

	// Build a request with BOTH header and cookie set
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/protected", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+headerToken)
	req.AddCookie(&http.Cookie{Name: "paper_session", Value: cookieToken})

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, parseErr := testutil.ParseJSONMap(resp)
	assert.NoError(t, parseErr)

	// The header token (user 42) should take priority over the cookie token (user 99)
	assert.Equal(t, float64(42), body["user_id"],
		"Authorization header should take priority over cookie")
	assert.Equal(t, "alice@example.com", body["email"])
}

// ---------------------------------------------------------------------------
// 2026-05-17 Wave 2 audit — finding M3 regression test.
// ---------------------------------------------------------------------------
//
// The JWT/access-token path MUST NOT populate is_super_admin Locals
// from the role claim. The Locals is the authoritative signal for
// assertSameTenant's cross-tenant bypass, so trusting the claim
// (which is only re-validated against the DB at the next request)
// would mean a demoted super_admin retains cross-tenant access for
// up to the token's TTL (24h).
//
// Only PermissionMiddleware.RequireSuperAdmin and the isAdmin helper
// — both DB re-checking — are authorized to set is_super_admin Locals.

// superAdminTestUser returns a user with role=super_admin used to
// craft a JWT that carries the role claim. The test then verifies
// that Protected does NOT set is_super_admin Locals from that claim.
func superAdminTestUser() *models.User {
	return &models.User{
		ID:        99,
		AccountID: 1,
		Email:     "ops@example.com",
		Name:      "Ops Person",
		Role:      "super_admin",
	}
}

// setupLocalsInspector mounts Protected() and a handler that
// serializes the relevant Locals back to the response. Tests use
// this to assert what the middleware did or did not set.
func setupLocalsInspector(userRepo *mocks.MockUserRepository) *fiber.App {
	app := testutil.SetupTestApp()
	// True nil interface when no repo is provided (avoid the typed-nil trap).
	var userRepoIface repository.UserRepository
	if userRepo != nil {
		userRepoIface = userRepo
	}
	authMW := middleware.NewAuthMiddleware(testJWTSecret, nil, userRepoIface, nil)

	app.Get("/inspect", authMW.Protected(), func(c *fiber.Ctx) error {
		out := fiber.Map{
			"user_id":   c.Locals("user_id"),
			"user_role": c.Locals("user_role"),
			"is_admin":  c.Locals("is_admin"),
		}
		// Surface the Locals as null when unset so the test can
		// distinguish absent (correct for is_super_admin from JWT)
		// from false (the wrong value to set).
		if v := c.Locals("is_super_admin"); v != nil {
			out["is_super_admin"] = v
		} else {
			out["is_super_admin"] = nil
		}
		return c.JSON(out)
	})
	return app
}

func TestProtected_JWTSuperAdminRole_DoesNotSetSuperAdminLocals(t *testing.T) {
	app := setupLocalsInspector(nil)

	token, err := auth.GenerateToken(superAdminTestUser(), testJWTSecret)
	assert.NoError(t, err)

	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/inspect", token, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := testutil.ParseJSONMap(resp)
	assert.NoError(t, err)

	// user_role is set from the claim (UI personalization signal).
	assert.Equal(t, "super_admin", body["user_role"])
	// is_admin is also set — it's a soft hint that RequireAdmin
	// re-checks against the DB. Not the authoritative gate.
	assert.Equal(t, true, body["is_admin"])
	// is_super_admin MUST be nil (Locals unset). If this assert
	// flips to anything else, a future code change has re-introduced
	// the demoted-super_admin-bypass window flagged in audit M3.
	assert.Nil(t, body["is_super_admin"],
		"is_super_admin Locals MUST be absent on the JWT path — only RequireSuperAdmin's DB re-check may set it")
}

// The access-token path mirrors the JWT path's role-derivation
// logic line-for-line (auth.go:142-152). The JWT regression test
// above is the canonical lock; we don't duplicate the same
// assertion under a different transport because the code paths are
// identical and a future change that re-introduces the
// JWT-claim-derived is_super_admin Locals would also be reflected
// in the access-token path. If the two paths ever diverge, copy
// the test.
