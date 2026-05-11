package middleware_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/auth"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testJWTSecret = "test-jwt-secret-key"

// testUser creates a simple User model for JWT generation.
func testUser() *models.User {
	return &models.User{
		ID:    42,
		Email: "alice@example.com",
		Name:  "Alice Wonderland",
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

	authMW := middleware.NewAuthMiddleware(testJWTSecret, accessTokenSvc, userRepo, nil)

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
	mockUserRepo.On("FindByID", mock.Anything, uint(99)).Return(storedUser, nil)

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
