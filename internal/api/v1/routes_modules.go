package v1

import "github.com/gofiber/fiber/v2"

// mountModuleContentGate installs the module lock / Mastery Paths gate
// (G061/G080) on the protected group. It must run before the content
// routes are declared, so Register calls it right after the group's
// global middleware.
func (r *Router) mountModuleContentGate(protected fiber.Router) {
	if r.ModuleProgressionHandler != nil {
		protected.Use(r.ModuleProgressionHandler.ContentGate())
	}
}

// registerModuleRoutes mounts the modules-lane endpoints: student module
// progress and completion actions (G061/G062), the teacher progress
// grid, External Tool item launches (G064) and course pacing blackout
// dates (G079).
func (r *Router) registerModuleRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	if h := r.ModuleProgressionHandler; h != nil {
		protected.Get("/courses/:course_id/module_progress", enrolled, h.GetProgress)
		protected.Get("/courses/:course_id/module_progressions", instructor, h.Roster)
		protected.Post("/courses/:course_id/modules/:module_id/items/:item_id/mark_read", enrolled, h.MarkItemRead)
		protected.Put("/courses/:course_id/modules/:module_id/items/:item_id/done", enrolled, h.MarkItemDone)
		protected.Delete("/courses/:course_id/modules/:module_id/items/:item_id/done", enrolled, h.UnmarkItemDone)
	}
	if r.ModuleItemHandler != nil {
		protected.Get("/courses/:course_id/modules/:module_id/items/:item_id/launch", enrolled, r.ModuleItemHandler.LaunchParams)
	}
	if h := r.CoursePaceHandler; h != nil {
		protected.Get("/courses/:course_id/course_pacing/:id/timeline", instructor, h.GetTimeline)
		protected.Get("/courses/:course_id/blackout_dates", instructor, h.ListBlackoutDates)
		protected.Post("/courses/:course_id/blackout_dates", instructor, h.CreateBlackoutDate)
		protected.Delete("/courses/:course_id/blackout_dates/:id", instructor, h.DeleteBlackoutDate)
	}
}
