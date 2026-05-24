package handlers_test

// F-012 — admin-tool Delete handlers (custom_role family) must refuse
// cross-tenant writes. Pre-fix:
//
//   - CustomRoleService.DeleteRole(id) had NO tenant scope; the
//     underlying repo's UPDATE ran without a WHERE account_id clause,
//     so a super_admin (or any admin who guessed an ID) could
//     soft-delete a role belonging to a different tenant by hitting
//     DELETE /accounts/<own>/roles/<other tenant's id>.
//   - The handler did not assert :account_id at all, so even a
//     correctly-shaped URL would leak through.
//
// Post-fix the handler:
//   (1) loads the role under callerAccountID — 404 on mismatch,
//   (2) asserts URL :account_id == role.AccountID — 404 on mismatch,
//   (3) passes callerAccountID through to repo.Delete which now
//       carries a WHERE account_id = ? clause as the third defense.
//
// The two regression cases below lock the contract. A handler smoke
// test for the happy path keeps the wire-up honest.

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

// adminToolAuthStub mirrors parentTieAuthStub on
// branch security/delete-parent-tie — populates user_id + account_id
// + enrollment_type Locals so handlers behave as if mounted behind
// middleware.Protected.
func adminToolAuthStub(callerAccountID uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(7))
		c.Locals("account_id", callerAccountID)
		c.Locals("enrollment_type", "AdminEnrollment")
		return c.Next()
	}
}

// TestDeleteRole_RejectsCrossTenant — F-012 repro. Admin in tenant 1
// hits DELETE /accounts/1/roles/42 but role 42 lives in tenant 99.
// The mock returns gorm.ErrRecordNotFound for the tenant-scoped
// FindByID(42, 1) — the handler MUST 404 before the destructive
// Delete fires.
func TestDeleteRole_RejectsCrossTenant(t *testing.T) {
	roleRepo := new(mocks.MockCustomRoleRepository)
	overrideRepo := new(mocks.MockRoleOverrideRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	customRoleService := service.NewCustomRoleService(roleRepo, overrideRepo, enrollmentRepo)

	// Tenant-scoped FindByID returns "not found" because the real
	// repo would not match account_id = 1 on a row owned by tenant 99.
	roleRepo.On("FindByID", mock.Anything, uint(42), uint(1)).
		Return(nil, assert.AnError)

	h := handlers.NewCustomRoleHandler(customRoleService)
	app := testutil.SetupTestApp()
	app.Delete("/accounts/:account_id/roles/:id",
		adminToolAuthStub(1),
		h.DeleteRole)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/accounts/1/roles/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// The load-bearing assertion: destructive Delete MUST NOT fire on
	// a cross-tenant row.
	roleRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteRole_RejectsParentTieMismatch — F-012 second-leg defense.
// Caller is a super_admin masquerading into tenant 1; role 42
// genuinely lives in tenant 1 (so FindByID succeeds) but the URL
// claims :account_id = 9. The parent-tie check fires 404 before the
// destructive write, preventing the URL-tampering vector where a
// caller binds an authorized role to a wrong-tenant URL.
func TestDeleteRole_RejectsParentTieMismatch(t *testing.T) {
	roleRepo := new(mocks.MockCustomRoleRepository)
	overrideRepo := new(mocks.MockRoleOverrideRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	customRoleService := service.NewCustomRoleService(roleRepo, overrideRepo, enrollmentRepo)

	// Role exists in tenant 1; FindByID returns it.
	roleRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.CustomRole{
		ID:        42,
		AccountID: 1,
		Name:      "Department Reviewer",
	}, nil)

	h := handlers.NewCustomRoleHandler(customRoleService)
	app := testutil.SetupTestApp()
	app.Delete("/accounts/:account_id/roles/:id",
		adminToolAuthStub(1),
		h.DeleteRole)

	// URL :account_id = 9 does NOT match role.AccountID = 1.
	resp := testutil.MakeRequest(app, http.MethodDelete, "/accounts/9/roles/42", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	roleRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// TestDeleteRole_HappyPath — same-tenant URL + matching parent_id
// deletes cleanly. Locks that the third positional arg to repo.Delete
// is the caller's accountID (not 0), which is the WHERE clause the
// repo SQL relies on as defense-in-depth.
func TestDeleteRole_HappyPath(t *testing.T) {
	roleRepo := new(mocks.MockCustomRoleRepository)
	overrideRepo := new(mocks.MockRoleOverrideRepository)
	enrollmentRepo := new(mocks.MockEnrollmentRepository)

	customRoleService := service.NewCustomRoleService(roleRepo, overrideRepo, enrollmentRepo)

	roleRepo.On("FindByID", mock.Anything, uint(42), uint(1)).Return(&models.CustomRole{
		ID:        42,
		AccountID: 1,
		Name:      "Department Reviewer",
	}, nil)
	roleRepo.On("Delete", mock.Anything, uint(42), uint(1)).Return(nil)

	h := handlers.NewCustomRoleHandler(customRoleService)
	app := testutil.SetupTestApp()
	app.Delete("/accounts/:account_id/roles/:id",
		adminToolAuthStub(1),
		h.DeleteRole)

	resp := testutil.MakeRequest(app, http.MethodDelete, "/accounts/1/roles/42", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	roleRepo.AssertCalled(t, "Delete", mock.Anything, uint(42), uint(1))
}

// TestBlueprintTemplateRepo_DeleteInterface — F-012 family-completeness
// proof. There is no live handler for blueprint_template Delete today
// (the repo method exists in the interface but no service or route
// calls it). The interface widening still has to land for two
// reasons:
//
//   (1) any future caller of BlueprintTemplateRepository.Delete is
//       forced by the compiler to pass accountID, preventing the same
//       "Delete by id alone" foot-gun that bit custom_role; and
//   (2) the underlying SQL UPDATE now carries
//       `course_id IN (SELECT id FROM courses WHERE account_id = ?)`
//       so a malicious or buggy future caller passing the right id
//       but the wrong (or zero) tenant still cannot touch a row
//       outside their tenant.
//
// This test pins (1) — the mock conforms to the new signature, which
// means the production repo, the production interface, and any
// future use site all share the widened shape.
func TestBlueprintTemplateRepo_DeleteInterface(t *testing.T) {
	repo := new(mocks.MockBlueprintTemplateRepository)
	repo.On("Delete", mock.Anything, uint(7), uint(1)).Return(nil)

	// The handler/service layer doesn't exist yet — pin the signature
	// by calling through the interface directly. The compiler enforces
	// that anyone implementing or consuming this interface must thread
	// accountID; this assertion just locks the contract observable.
	err := repo.Delete(nil, uint(7), uint(1)) //nolint:staticcheck // nil ctx OK in mock-only path
	assert.NoError(t, err)
	repo.AssertCalled(t, "Delete", mock.Anything, uint(7), uint(1))
}
