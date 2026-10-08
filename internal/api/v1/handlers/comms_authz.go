package handlers

import "github.com/gofiber/fiber/v2"

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
// the caller is an admin or an active teacher/TA of the course (in the
// caller's tenant). Used to pick behavior (e.g. BBB moderator vs viewer),
// not to refuse requests — refusals go through ResourceAuthorizer.
func callerIsCourseInstructor(c *fiber.Ctx, a *ResourceAuthorizer, courseID uint) bool {
	if a == nil {
		return false
	}
	if a.isAdmin(c) {
		return true
	}
	userID, ok := c.Locals("user_id").(uint)
	if !ok || userID == 0 {
		return false
	}
	e, err := a.enrollmentRepo.FindByUserAndCourse(c.Context(), userID, courseID, callerAccountID(c))
	if err != nil || e == nil || e.WorkflowState != "active" {
		return false
	}
	return e.Type == "TeacherEnrollment" || e.Type == "TaEnrollment"
}

// callerIsAdmin is the DB-backed admin check (the JWT is_admin Locals is
// only a soft UI hint — a demoted admin's unexpired token still carries
// it). Delegates to the ResourceAuthorizer so the admin definition stays
// in one place.
func callerIsAdmin(c *fiber.Ctx, a *ResourceAuthorizer) bool {
	if a == nil {
		return false
	}
	return a.isAdmin(c)
}
