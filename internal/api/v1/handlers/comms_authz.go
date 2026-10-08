package handlers

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/gofiber/fiber/v2"
)

// authzDenied reports whether a ResourceAuthorizer check refused the
// request. It is true when the check returned an error OR when it already
// wrote a 403/404 response and returned nil (the pre-fix authzForbidden
// shape). Callers short-circuit with `return err` — either the error the
// authorizer produced or nil after a response was already written — so
// the handler never falls through to the protected work.
func authzDenied(c *fiber.Ctx, err error) bool {
	if err != nil {
		return true
	}
	switch c.Response().StatusCode() {
	case fiber.StatusForbidden, fiber.StatusNotFound:
		return true
	}
	return false
}

// callerIsCourseInstructor reports, WITHOUT writing a response, whether
// the caller is an admin of the course's tenant or an active teacher/TA of
// the course. Used to pick behavior (e.g. BBB moderator vs viewer), not to
// refuse requests — refusals go through ResourceAuthorizer.
func callerIsCourseInstructor(c *fiber.Ctx, a *ResourceAuthorizer, courseID uint) bool {
	if a == nil {
		return false
	}
	return a.RequireCourseInstructor(c, courseID) == nil
}

// callerIsAdmin is the DB-backed admin check (the JWT is_admin Locals is
// only a soft UI hint — a demoted admin's unexpired token still carries
// it). Uses middleware.EffectiveRole so a masquerade session is capped at
// the real user's role, like every other admin check.
func callerIsAdmin(c *fiber.Ctx, a *ResourceAuthorizer) bool {
	if a == nil {
		return false
	}
	switch middleware.EffectiveRole(c, a.userRepo) {
	case middleware.RoleAdmin, middleware.RoleSuperAdmin:
		return true
	}
	return false
}
