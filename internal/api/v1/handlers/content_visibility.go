package handlers

import "github.com/gofiber/fiber/v2"

// canViewUnpublishedContent reports whether the caller may see draft
// (unpublished) course content — pages, modules and module items. Canvas
// shows drafts to users who can manage course content (teachers, TAs,
// designers, admins) and hides them from students and observers.
//
// It reads the Locals the course middleware (RequireEnrolled /
// RequireCourseRole) stamps: is_admin for admins, enrollment_type for
// everyone else. A request with neither is treated as a learner — the
// safe default.
func canViewUnpublishedContent(c *fiber.Ctx) bool {
	if isSuper, _ := c.Locals("is_super_admin").(bool); isSuper {
		return true
	}
	if isAdmin, _ := c.Locals("is_admin").(bool); isAdmin {
		return true
	}
	switch enrollmentType, _ := c.Locals("enrollment_type").(string); enrollmentType {
	case "TeacherEnrollment", "TaEnrollment", "DesignerEnrollment":
		return true
	}
	return false
}
