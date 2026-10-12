package handlers_test

// G114 / G115 / G117 / G119 / G121 — account administration handlers:
// tenant scoping (404 across tenants) and the super_admin protection.
// All data is made up.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
)

func adminAccountsApp(t *testing.T, g *gorm.DB) *fiber.App {
	t.Helper()
	obs := service.NewObserverService(postgres.NewEnrollmentRepository(g), postgres.NewCourseRepository(g), postgres.NewUserRepository(g)).
		SetObserverLinkStore(postgres.NewObserverLinkRepository(g))
	h := handlers.NewAdminAccountsHandler(handlers.AdminAccountsDeps{
		Users:     service.NewAdminUserService(g),
		Logins:    service.NewUserLoginService(g),
		Observers: obs,
		Reports:   service.NewAccountReportService(g, nil).SetAsync(false),
		Brand:     service.NewBrandConfigService(g),
		Tools:     service.NewAdminToolsService(g),
		Quotas:    service.NewStorageQuotaService(g),
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// Account admin 3 of account 2 (see coreScratchDB).
		c.Locals("user_id", uint(3))
		c.Locals("account_id", uint(2))
		c.Locals("is_admin", true)
		c.Locals("is_super_admin", false)
		return c.Next()
	})
	app.Post("/api/v1/accounts/:account_id/users", h.CreateUser)
	app.Get("/api/v1/accounts/:account_id/users/:user_id", h.GetUserDetail)
	app.Put("/api/v1/accounts/:account_id/users/:user_id", h.UpdateUser)
	app.Post("/api/v1/accounts/:account_id/users/:user_id/logins", h.CreateLogin)
	app.Put("/api/v1/accounts/:account_id/users/:user_id/observees/:observee_id", h.LinkObservee)
	app.Post("/api/v1/accounts/:account_id/reports/:report", h.StartReport)
	app.Get("/api/v1/accounts/:account_id/reports/:report/:id/download", h.DownloadReport)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body interface{}) (int, map[string]interface{}) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestAdminAccounts_TenantScopeAndSuperAdminGuard(t *testing.T) {
	g := coreScratchDB(t)
	app := adminAccountsApp(t, g)
	users := postgres.NewUserRepository(g)
	root := &models.User{ID: 20, AccountID: 2, Name: "Root Example", LoginID: "root@example.test", Email: "root@example.test", PasswordHash: "x", Role: "super_admin"}
	kid := &models.User{ID: 21, AccountID: 2, Name: "Kid Example", LoginID: "kid@example.test", Email: "kid@example.test", PasswordHash: "x", Role: "user"}
	for _, u := range []*models.User{root, kid} {
		require.NoError(t, users.Create(context.Background(), u))
	}

	// User 8 lives in account 1: invisible from account 2.
	code, _ := doJSON(t, app, http.MethodGet, "/api/v1/accounts/2/users/8", nil)
	assert.Equal(t, http.StatusNotFound, code)
	// The URL can't name another account either.
	code, _ = doJSON(t, app, http.MethodGet, "/api/v1/accounts/1/users/8", nil)
	assert.Equal(t, http.StatusNotFound, code)

	code, created := doJSON(t, app, http.MethodPost, "/api/v1/accounts/2/users", fiber.Map{"user": fiber.Map{"name": "New Person", "email": "new@example.test"}})
	require.Equal(t, http.StatusCreated, code, created)
	assert.Equal(t, float64(2), created["account_id"])
	assert.Equal(t, true, created["requires_password_reset"])

	// Login collision is a 409.
	code, _ = doJSON(t, app, http.MethodPost, "/api/v1/accounts/2/users", fiber.Map{"user": fiber.Map{"name": "Dup", "email": "dup@example.test", "login_id": "JANE@example.test"}})
	assert.Equal(t, http.StatusConflict, code)

	// An account admin can't edit or add logins to a super_admin.
	code, _ = doJSON(t, app, http.MethodPut, "/api/v1/accounts/2/users/20", fiber.Map{"user": fiber.Map{"email": "evil@example.test"}})
	assert.Equal(t, http.StatusForbidden, code)
	code, _ = doJSON(t, app, http.MethodPost, "/api/v1/accounts/2/users/20/logins", fiber.Map{"login": fiber.Map{"unique_id": "backdoor"}})
	assert.Equal(t, http.StatusForbidden, code)

	// Detail of a manageable user includes the sections the page needs.
	code, detail := doJSON(t, app, http.MethodGet, "/api/v1/accounts/2/users/21", nil)
	require.Equal(t, http.StatusOK, code)
	for _, k := range []string{"enrollments", "logins", "observees", "observers", "permissions"} {
		assert.Contains(t, detail, k)
	}

	// Observer links stay inside the tenant.
	code, _ = doJSON(t, app, http.MethodPut, "/api/v1/accounts/2/users/21/observees/8", nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = doJSON(t, app, http.MethodPut, "/api/v1/accounts/2/users/3/observees/21", nil)
	assert.Equal(t, http.StatusCreated, code)

	// Report runs are per-tenant.
	other, err := service.NewAccountReportService(g, nil).SetAsync(false).Start(context.Background(), 1, 8, service.ReportUnpublishedCourse, service.AccountReportParams{})
	require.NoError(t, err)
	code, _ = doJSON(t, app, http.MethodGet, "/api/v1/accounts/2/reports/unpublished_courses_csv/"+adminItoa(other.ID)+"/download", nil)
	assert.Equal(t, http.StatusNotFound, code)
	code, run := doJSON(t, app, http.MethodPost, "/api/v1/accounts/2/reports/unpublished_courses_csv", nil)
	require.Equal(t, http.StatusCreated, code, run)
	assert.Equal(t, "complete", run["status"])
	code, _ = doJSON(t, app, http.MethodPost, "/api/v1/accounts/2/reports/not_a_report", nil)
	assert.Equal(t, http.StatusNotFound, code)
}

func adminItoa(v uint) string {
	b, _ := json.Marshal(v)
	return string(b)
}
