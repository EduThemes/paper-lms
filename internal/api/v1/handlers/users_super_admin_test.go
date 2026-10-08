package handlers_test

// B175 regression: an account admin must not be able to demote, suspend
// or force-reset a super_admin. A super_admin may.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func superAdminTargetApp(callerIsSuper bool) (*fiber.App, *mocks.MockUserRepository) {
	repo := new(mocks.MockUserRepository)
	h := handlers.NewUserHandler(service.NewUserService(repo), "test-jwt-secret", "test", nil, nil, nil)
	app := testutil.SetupTestApp()
	auth := func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(2))
		c.Locals("account_id", uint(1))
		c.Locals("is_admin", true)
		if callerIsSuper {
			c.Locals("is_super_admin", true)
		}
		return c.Next()
	}
	app.Put("/users/:id/role", auth, h.UpdateUserRole)
	app.Put("/users/:id/suspension", auth, h.UpdateUserSuspension)
	app.Post("/users/:id/force_password_reset", auth, h.ForcePasswordReset)
	app.Post("/users/bulk_suspension", auth, h.BulkUpdateSuspension)
	repo.On("FindByID", mock.Anything, uint(9), uint(1)).
		Return(&models.User{ID: 9, AccountID: 1, Role: models.RoleSuperUser, Email: "ops@example.com"}, nil)
	return app, repo
}

func TestAccountAdminCannotManageSuperAdmin(t *testing.T) {
	app, repo := superAdminTargetApp(false)

	resp := testutil.MakeRequest(app, http.MethodPut, "/users/9/role", testutil.JSONBody(map[string]any{"role": "user"}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, "/users/9/suspension", testutil.JSONBody(map[string]any{"suspended": true}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, "/users/9/force_password_reset", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, "/users/bulk_suspension",
		testutil.JSONBody(map[string]any{"user_ids": []uint{9}, "suspended": true}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.EqualValues(t, 0, body["count"])

	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestSuperAdminCanManageSuperAdmin(t *testing.T) {
	app, repo := superAdminTargetApp(true)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.User")).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPut, "/users/9/role", testutil.JSONBody(map[string]any{"role": "admin"}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
