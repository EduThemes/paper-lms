package v1

import "github.com/gofiber/fiber/v2"

// registerLearnerOutcomesRoutes mounts the learner-outcomes lane surfaces:
// course analytics (G154), the student Learning Mastery view (G152) and
// ePortfolio additions (G155). Registered once from Register.
func (r *Router) registerLearnerOutcomesRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	// Course analytics (G154).
	if r.AnalyticsHandler != nil {
		protected.Get("/courses/:course_id/analytics/activity/categories", instructor, r.AnalyticsHandler.GetCourseActivityCategories)
		// The client reports time on page for a page view it created.
		protected.Put("/page_views/:id", r.AnalyticsHandler.UpdatePageView)
	}
}
