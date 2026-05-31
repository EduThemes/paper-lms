package handlers_test

// SEC-005: the SIS CSV export endpoints discarded the URL :account_id and the
// service dumped every tenant's users/courses/sections/enrollments. The fix
// scopes the export to the caller's tenant and rejects a cross-tenant
// :account_id via assertSameTenant (404, existence-leak contract). These tests
// lock the cross-tenant rejection. The rejection short-circuits BEFORE the
// service is invoked, so a nil service is sufficient (and proves the guard runs
// first). Same-tenant scoping of the SQL queries is exercised separately at the
// repository/DB layer.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

// sisAdminAuthStub authenticates the caller as an account admin of
// callerAccountID (is_admin true, is_super_admin unset — a regular account
// admin, which must NOT cross tenant boundaries).
func sisAdminAuthStub(callerAccountID uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("is_admin", true)
		return c.Next()
	}
}

func TestExportUsersCSV_RejectsCrossTenant(t *testing.T) {
	h := handlers.NewSISImportHandler(nil) // service never reached on the 404 path
	app := testutil.SetupTestApp()
	app.Get("/accounts/:account_id/sis_exports/users.csv", sisAdminAuthStub(1), h.ExportUsersCSV)

	// Admin of account 1 requests account 2's export → 404.
	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/sis_exports/users.csv", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestExportEnrollmentsCSV_RejectsCrossTenant(t *testing.T) {
	h := handlers.NewSISImportHandler(nil)
	app := testutil.SetupTestApp()
	app.Get("/accounts/:account_id/sis_exports/enrollments.csv", sisAdminAuthStub(1), h.ExportEnrollmentsCSV)

	resp := testutil.MakeRequest(app, http.MethodGet, "/accounts/2/sis_exports/enrollments.csv", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
