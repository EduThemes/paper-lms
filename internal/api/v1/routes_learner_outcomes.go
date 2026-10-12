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

	// Student Learning Mastery view (G152). `enrolled` admits the course's
	// members; the handler limits students to themselves and observers to
	// their linked students (user_id may be "self").
	if r.OutcomeProficiencyHandler != nil {
		protected.Get("/courses/:course_id/outcome_mastery/users/:user_id", enrolled, r.OutcomeProficiencyHandler.StudentMastery)
	}

	// ePortfolios (G155). Owner-or-admin / viewer checks are in the
	// handlers (portfolios are user-owned, not course-scoped).
	if r.PortfolioHandler != nil {
		protected.Post("/portfolios/:id/share_link", r.PortfolioHandler.CreateShareLink)
		protected.Delete("/portfolios/:id/share_link", r.PortfolioHandler.RevokeShareLink)
		protected.Delete("/portfolios/:id/comments/:comment_id", r.PortfolioHandler.DeleteComment)
		protected.Get("/portfolios/:id/importable_submissions", r.PortfolioHandler.ListImportableSubmissions)
	}
}

// registerLearnerOutcomesPublicRoutes mounts the lane's unauthenticated
// endpoints: a privately shared ePortfolio (the secret link is the
// permission) and its files. Must run before the protected group is
// mounted on /api/v1.
func (r *Router) registerLearnerOutcomesPublicRoutes(api fiber.Router) {
	if r.PortfolioHandler != nil {
		api.Get("/portfolios/shared/:token", r.PortfolioHandler.GetSharedPortfolio)
		api.Get("/portfolios/shared/:token/files/:file_id", r.PortfolioHandler.DownloadSharedArtifactFile)
	}
}
