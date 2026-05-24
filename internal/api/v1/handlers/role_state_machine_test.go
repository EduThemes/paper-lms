package handlers

import (
	"testing"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/gofiber/fiber/v2"
)

// callerRoleForStateMachine is the runtime hinge of the F-018 state
// machine — every TransitionXxxWorkflowState helper trusts the role
// string it returns. The PR body's load-bearing claim is the
// precedence order:
//
//	is_super_admin > is_admin > enrollment_type
//
// These tests lock that contract. If the precedence is silently
// reversed, an enrolled-as-teacher super_admin would be tagged
// "teacher" and lose the cross-tenant transitions super_admin is
// supposed to retain.

func TestCallerRoleForStateMachine_SuperAdminBeatsAdmin(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("is_super_admin", true)
		c.Locals("is_admin", true)
		c.Locals("enrollment_type", "StudentEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleSuperUser {
			t.Errorf("expected super_admin, got %q (is_super_admin should beat is_admin)", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_AdminBeatsEnrollmentType(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("is_admin", true)
		c.Locals("enrollment_type", "StudentEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleAdmin {
			t.Errorf("expected admin, got %q (is_admin should beat enrollment_type)", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_TeacherEnrollment(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", "TeacherEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleTeacher {
			t.Errorf("expected teacher, got %q", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_TAEnrollment(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", "TaEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleTA {
			t.Errorf("expected ta, got %q", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_StudentEnrollment(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", "StudentEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleStudent {
			t.Errorf("expected student, got %q", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_ObserverEnrollment(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", "ObserverEnrollment")
		role := callerRoleForStateMachine(c)
		if role != models.RoleObserver {
			t.Errorf("expected observer, got %q", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}

func TestCallerRoleForStateMachine_NoLocalsReturnsEmpty(t *testing.T) {
	app := testutil.SetupTestApp()

	app.Get("/probe", func(c *fiber.Ctx) error {
		role := callerRoleForStateMachine(c)
		if role != "" {
			t.Errorf("expected empty string when no role Locals set, got %q", role)
		}
		return c.SendStatus(200)
	})
	_ = testutil.MakeRequest(app, fiber.MethodGet, "/probe", nil)
}
