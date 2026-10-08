package v1

import "github.com/gofiber/fiber/v2"

// registerGroupsConferencesRoutes mounts the groups-conferences parity
// lane: conference provider discovery and recording management (G098),
// and the group-set tools — unassigned roster, auto-assign, bulk group
// creation, CSV import / export (G100). The handlers authorize against the
// set's / conference's course themselves (instructor), since the ids are
// not course-scoped in the URL.
func (r *Router) registerGroupsConferencesRoutes(protected fiber.Router, enrolled fiber.Handler) {
	// Conferences (G098)
	protected.Get("/courses/:course_id/conferences/providers", enrolled, r.ConferenceHandler.ListProviders)
	protected.Delete("/conferences/:id/recordings/:recording_id", r.ConferenceHandler.DeleteRecording)

	// Group sets (G100) — Canvas-compatible paths.
	protected.Get("/group_categories/:group_category_id/users", r.GroupHandler.ListGroupCategoryUsers)
	protected.Post("/group_categories/:group_category_id/assign_unassigned_members", r.GroupHandler.AssignUnassignedMembers)
	protected.Post("/group_categories/:group_category_id/create_groups", r.GroupHandler.CreateGroupsInCategory)
	protected.Post("/group_categories/:group_category_id/import", r.GroupHandler.ImportGroupCategoryCSV)
	protected.Get("/group_categories/:group_category_id/export", r.GroupHandler.ExportGroupCategoryCSV)
}
