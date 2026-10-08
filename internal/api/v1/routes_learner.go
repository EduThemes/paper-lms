package v1

import "github.com/gofiber/fiber/v2"

// registerLearnerRoutes mounts the student / observer experience
// (learner-dashboard lane): planner, dashboard customization, activity
// stream, missing submissions, the grade summary, observer alerts and the
// user profile. Every route is self-scoped (the handler reads the caller
// from Locals) except the grade summary, which sits behind the course
// enrollment gate, and the observee-aware missing-submissions route,
// whose handler checks the observer link.
func (r *Router) registerLearnerRoutes(protected fiber.Router, enrolled fiber.Handler) {
	h := r.LearnerHandler
	if h == nil {
		return
	}

	// Planner / to-do list (Canvas planner API).
	protected.Get("/planner/items", h.PlannerItems)
	protected.Get("/planner_notes", h.ListPlannerNotes)
	protected.Post("/planner_notes", h.CreatePlannerNote)
	protected.Put("/planner_notes/:id", h.UpdatePlannerNote)
	protected.Delete("/planner_notes/:id", h.DeletePlannerNote)
	protected.Get("/planner/overrides", h.ListPlannerOverrides)
	protected.Post("/planner/overrides", h.CreatePlannerOverride)
	protected.Put("/planner/overrides/:id", h.UpdatePlannerOverride)
	protected.Delete("/planner/overrides/:id", h.DeletePlannerOverride)

	// Dashboard cards and per-user course customization.
	protected.Get("/dashboard/dashboard_cards", h.DashboardCards)
	protected.Put("/dashboard/positions", h.SetDashboardPositions)
	protected.Get("/users/self/favorites/courses", h.ListFavoriteCourses)
	protected.Post("/users/self/favorites/courses/:course_id", h.AddFavoriteCourse)
	protected.Delete("/users/self/favorites/courses/:course_id", h.RemoveFavoriteCourse)
	protected.Get("/users/self/course_nicknames", h.ListCourseNicknames)
	protected.Put("/users/self/course_nicknames/:course_id", h.SetCourseNickname)
	protected.Delete("/users/self/course_nicknames/:course_id", h.DeleteCourseNickname)
	protected.Get("/users/self/colors", h.GetCustomColors)
	protected.Put("/users/self/colors/:asset_string", h.SetCustomColor)

	// Activity stream and missing work.
	protected.Get("/users/self/activity_stream", h.ActivityStream)
	protected.Get("/users/self/activity_stream/summary", h.ActivityStreamSummary)
	protected.Get("/users/:user_id/missing_submissions", h.MissingSubmissions)

	// Student grade summary (self / observer / staff).
	protected.Get("/courses/:course_id/grade_summary", enrolled, h.GradeSummary)

	// Observer alerts.
	protected.Get("/observer_alert_thresholds", h.ListAlertThresholds)
	protected.Post("/observer_alert_thresholds", h.CreateAlertThreshold)
	protected.Put("/observer_alert_thresholds/:id", h.UpdateAlertThreshold)
	protected.Delete("/observer_alert_thresholds/:id", h.DeleteAlertThreshold)
	protected.Get("/users/self/observer_alerts", h.ListObserverAlerts)
	protected.Get("/users/self/observer_alerts/unread_count", h.ObserverAlertsUnreadCount)
	protected.Put("/observer_alerts/:id/:workflow_state", h.UpdateObserverAlert)

	// Profile: pronouns, bio, links, avatar.
	protected.Put("/users/self/profile", h.UpdateProfile)
	protected.Post("/users/self/avatar", h.UploadAvatar)
	protected.Delete("/users/self/avatar", h.DeleteAvatar)
	protected.Get("/users/:id/profile_card", h.GetProfileCard)
	protected.Get("/users/:id/avatar", h.GetAvatar)
}
