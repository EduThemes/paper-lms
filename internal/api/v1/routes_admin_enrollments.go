package v1

import "github.com/gofiber/fiber/v2"

// registerAdminEnrollmentRoutes mounts the enrollment lifecycle, course
// invitation, section management and cross-listing endpoints
// (admin-enrollments lane: G105, G107, G108, G109, G122).
//
//   - Enrollment edits and lifecycle tasks are course-gated to
//     instructors; the handler then applies Canvas's add-role rules
//     (TAs manage only students and observers) and section limits.
//   - Accept / reject carry no course gate: the invitee has no active
//     enrollment yet. The handler requires the enrollment to be the
//     caller's own (404 otherwise).
//   - /sections/:id routes have no :course_id; the handler authorizes
//     against the section's course (teacher or admin) and, for
//     cross-listing, the destination course too.
func (r *Router) registerAdminEnrollmentRoutes(protected fiber.Router, instructor fiber.Handler) {
	protected.Put("/courses/:course_id/enrollments/:id", instructor, r.EnrollmentHandler.UpdateEnrollment)
	protected.Delete("/courses/:course_id/enrollments/:id", instructor, r.EnrollmentHandler.DeleteEnrollment)
	protected.Put("/courses/:course_id/enrollments/:id/reactivate", instructor, r.EnrollmentHandler.ReactivateEnrollment)
	protected.Post("/courses/:course_id/enrollments/:id/accept", r.EnrollmentHandler.AcceptInvitation)
	protected.Post("/courses/:course_id/enrollments/:id/reject", r.EnrollmentHandler.RejectInvitation)
	protected.Get("/users/self/course_invitations", r.EnrollmentHandler.ListMyInvitations)

	protected.Put("/sections/:id", r.SectionHandler.UpdateSection)
	protected.Delete("/sections/:id", r.SectionHandler.DeleteSection)
	protected.Post("/sections/:id/crosslist/:new_course_id", r.SectionHandler.CrosslistSection)
	protected.Delete("/sections/:id/crosslist", r.SectionHandler.UncrosslistSection)
}
