package middleware_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B085 / B183: the access-token path enforces developer-key scopes and
// refuses tokens whose developer key is no longer active.
func TestProtected_EnforcesAccessTokenScopes(t *testing.T) {
	const raw = "scoped-raw-token"
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])

	newApp := func(token *models.AccessToken) *fiber.App {
		tokenRepo := new(mocks.MockAccessTokenRepository)
		tokenRepo.On("FindByToken", mock.Anything, hash).Return(token, nil)
		tokenRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
		userRepo := new(mocks.MockUserRepository)
		userRepo.On("FindByID", mock.Anything, uint(42), uint(0)).Return(&models.User{ID: 42, AccountID: 1, Role: "user"}, nil)

		authMW := middleware.NewAuthMiddleware(testJWTSecret, service.NewAccessTokenService(tokenRepo), userRepo, nil)
		app := testutil.SetupTestApp()
		ok := func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
		app.Get("/api/v1/courses/:course_id/assignments", authMW.Protected(), ok)
		app.Delete("/api/v1/courses/:course_id", authMW.Protected(), ok)
		return app
	}

	keyID := uint(5)
	future := time.Now().Add(time.Hour)
	scoped := func(keyState string) *models.AccessToken {
		return &models.AccessToken{
			ID: 1, UserID: 42, DeveloperKeyID: &keyID, WorkflowState: "active", ExpiresAt: &future,
			Scopes:       `["url:GET|/api/v1/courses/:course_id/assignments"]`,
			DeveloperKey: &models.DeveloperKey{ID: 5, RequireScopes: true, WorkflowState: keyState},
		}
	}

	app := newApp(scoped("active"))
	resp := testutil.MakeAuthenticatedRequest(app, http.MethodGet, "/api/v1/courses/3/assignments", raw, nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "in-scope call")
	resp = testutil.MakeAuthenticatedRequest(app, http.MethodDelete, "/api/v1/courses/3", raw, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "out-of-scope call must be refused")

	inactive := newApp(scoped("inactive"))
	resp = testutil.MakeAuthenticatedRequest(inactive, http.MethodGet, "/api/v1/courses/3/assignments", raw, nil)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "token of a deactivated key must stop working")
}
