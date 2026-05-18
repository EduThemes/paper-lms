package v1

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/gofiber/fiber/v2"
)

// registerP3FeatureRoutes mounts the P3 feature surfaces: feature
// flags (account/course/user scopes), custom gradebook columns,
// mastery paths, appointment groups (scheduler), outcome proficiency,
// discussion checkpoints (multi-deadline thread participation), smart
// search (pgvector cosine similarity), Commons content library, and
// AI assist proxy.
func (r *Router) registerP3FeatureRoutes(protected fiber.Router, admin, enrolled, instructor fiber.Handler) {
	// Feature Flags — Canvas-compatible API
	// Account-scoped (admin only)
	protected.Get("/accounts/:id/features", admin, r.FeatureFlagHandler.ListAccountFeatures)
	protected.Get("/accounts/:id/features/:feature", admin, r.FeatureFlagHandler.GetAccountFeature)
	protected.Put("/accounts/:id/features/:feature", admin, r.FeatureFlagHandler.SetAccountFeature)
	protected.Delete("/accounts/:id/features/:feature", admin, r.FeatureFlagHandler.DeleteAccountFeature)
	// Course-scoped (any enrolled user can read; teacher/admin can write)
	protected.Get("/courses/:id/features", enrolled, r.FeatureFlagHandler.ListCourseFeatures)
	protected.Get("/courses/:id/features/:feature", enrolled, r.FeatureFlagHandler.GetCourseFeature)
	protected.Put("/courses/:id/features/:feature", instructor, r.FeatureFlagHandler.SetCourseFeature)
	protected.Delete("/courses/:id/features/:feature", instructor, r.FeatureFlagHandler.DeleteCourseFeature)
	// Per-user (always self)
	protected.Get("/users/self/features", r.FeatureFlagHandler.ListUserFeatures)
	protected.Get("/users/self/features/:feature", r.FeatureFlagHandler.GetUserFeature)
	protected.Put("/users/self/features/:feature", r.FeatureFlagHandler.SetUserFeature)
	protected.Delete("/users/self/features/:feature", r.FeatureFlagHandler.DeleteUserFeature)

	// Custom Gradebook Columns (instructor-only)
	protected.Get("/courses/:id/custom_gradebook_columns", instructor, r.CustomGradebookColumnHandler.List)
	protected.Post("/courses/:id/custom_gradebook_columns", instructor, r.CustomGradebookColumnHandler.Create)
	protected.Put("/courses/:id/custom_gradebook_columns/:column_id", instructor, r.CustomGradebookColumnHandler.Update)
	protected.Delete("/courses/:id/custom_gradebook_columns/:column_id", instructor, r.CustomGradebookColumnHandler.Delete)
	protected.Post("/courses/:id/custom_gradebook_columns/reorder", instructor, r.CustomGradebookColumnHandler.Reorder)
	protected.Get("/courses/:id/custom_gradebook_columns/:column_id/data", instructor, r.CustomGradebookColumnHandler.ListData)
	protected.Put("/courses/:id/custom_gradebook_columns/:column_id/data/:user_id", instructor, r.CustomGradebookColumnHandler.SetCell)
	protected.Put("/courses/:id/custom_gradebook_columns/data", instructor, r.CustomGradebookColumnHandler.BulkUpdate)

	// Mastery Paths (Conditional Release) — instructor-only management
	protected.Get("/courses/:course_id/mastery_paths/rules", instructor, r.MasteryPathHandler.ListRules)
	protected.Get("/courses/:course_id/mastery_paths/rules/:assignment_id", instructor, r.MasteryPathHandler.GetRuleForAssignment)
	protected.Post("/courses/:course_id/mastery_paths/rules", instructor, r.MasteryPathHandler.CreateRule)
	protected.Put("/courses/:course_id/mastery_paths/rules/:rule_id", instructor, r.MasteryPathHandler.ReplaceRule)
	protected.Delete("/courses/:course_id/mastery_paths/rules/:rule_id", instructor, r.MasteryPathHandler.DeleteRule)

	// Appointment Groups (Scheduler) — Canvas-compatible
	protected.Get("/courses/:course_id/appointment_groups", enrolled, r.AppointmentGroupHandler.List)
	protected.Post("/courses/:course_id/appointment_groups", enrolled, r.AppointmentGroupHandler.Create)
	protected.Get("/appointment_groups", r.AppointmentGroupHandler.List) // accepts ?course_id=
	protected.Get("/appointment_groups/:id", r.AppointmentGroupHandler.Get)
	protected.Put("/appointment_groups/:id", r.AppointmentGroupHandler.Update)
	protected.Delete("/appointment_groups/:id", r.AppointmentGroupHandler.Delete)
	protected.Get("/appointment_groups/:id/appointments", r.AppointmentGroupHandler.ListSlots)
	protected.Get("/appointment_groups/:id/appointments/:slot_id/reservations", r.AppointmentGroupHandler.ListReservations)
	protected.Post("/appointment_groups/:id/appointments/:slot_id/reservations", r.AppointmentGroupHandler.Reserve)
	protected.Delete("/appointment_groups/:id/appointments/:slot_id/reservations/:reservation_id", r.AppointmentGroupHandler.CancelReservation)

	// Outcome Proficiency — Account scope
	protected.Get("/accounts/:id/outcome_proficiency", admin, r.OutcomeProficiencyHandler.GetForAccount)
	protected.Post("/accounts/:id/outcome_proficiency", admin, r.OutcomeProficiencyHandler.SetForAccount)
	protected.Delete("/accounts/:id/outcome_proficiency", admin, r.OutcomeProficiencyHandler.DeleteForAccount)
	// Outcome Proficiency — Course scope
	protected.Get("/courses/:id/outcome_proficiency", enrolled, r.OutcomeProficiencyHandler.GetForCourse)
	protected.Post("/courses/:id/outcome_proficiency", instructor, r.OutcomeProficiencyHandler.SetForCourse)
	protected.Delete("/courses/:id/outcome_proficiency", instructor, r.OutcomeProficiencyHandler.DeleteForCourse)
	// Learning Mastery Gradebook
	protected.Get("/courses/:id/learning_mastery_gradebook", instructor, r.OutcomeProficiencyHandler.LearningMasteryGradebook)

	// Discussion Checkpoints (Canvas-compatible multi-deadline thread participation)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/checkpoints", enrolled, r.DiscussionCheckpointHandler.ListCheckpoints)
	protected.Post("/courses/:course_id/discussion_topics/:topic_id/checkpoints", instructor, r.DiscussionCheckpointHandler.CreateCheckpoints)
	protected.Get("/courses/:course_id/discussion_topics/:topic_id/checkpoints/progress", enrolled, r.DiscussionCheckpointHandler.GetUserProgress)
	protected.Put("/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id", instructor, r.DiscussionCheckpointHandler.UpdateCheckpoint)
	protected.Delete("/courses/:course_id/discussion_topics/:topic_id/checkpoints/:id", instructor, r.DiscussionCheckpointHandler.DeleteCheckpoint)

	// Smart Search (pgvector cosine similarity).
	r.mountSmartSearchRoutes(protected, enrolled, instructor)

	// Commons content library (district-scoped sharing).
	// IMPORTANT: register /commons/favorites BEFORE /commons/:id so the literal
	// path wins over the wildcard.
	protected.Get("/commons/favorites", r.CommonsHandler.ListFavorites)
	protected.Get("/commons", r.CommonsHandler.Browse)
	protected.Get("/commons/:id", r.CommonsHandler.Get)
	protected.Post("/commons/:id/favorite", r.CommonsHandler.Favorite)
	protected.Post("/commons/:id/import", r.CommonsHandler.Import)
	protected.Post("/courses/:course_id/commons/publish", instructor, r.CommonsHandler.Publish)

	// AI Assist proxy for RCE V2 toolbar (Anthropic Messages API).
	// Per-user rate limit (30 / 5 min) is the cost gate — any authenticated user
	// can call it, but no single account can drain the API budget.
	protected.Post("/ai_assist/:action", middleware.AIAssistRateLimit(), r.AIAssistHandler.Dispatch)
}

// mountSmartSearchRoutes wires the smart-search surface onto `protected`.
//
// The Search endpoint is always available — it reads from the existing
// content_embeddings table and works the moment any row is indexed (by
// Reindex, future webhooks, or background workers).
//
// The Reindex endpoint is GATED on the handler having a non-nil
// ReindexSourceLister wired in main.go. Without sources the handler
// can only ever respond 501 Not Implemented (see
// `internal/api/v1/handlers/smart_search.go` Reindex), so we'd be
// advertising an unimplemented endpoint and inviting client code that
// retries on 501. Mirror the same shape as `if r.GamificationHandler
// == nil { return }` in `routes_gamification.go`: when the dependency
// isn't wired, the route simply does not exist (404), not 501.
//
// To enable Reindex: pass a non-nil ReindexSourceLister to
// `handlers.NewSmartSearchHandler` in `cmd/server/main.go`.
//
// This is a separate method (rather than inline) so the gating
// decision is unit-testable in `routes_smart_search_test.go` without
// having to stand up the rest of the P3 feature handlers.
func (r *Router) mountSmartSearchRoutes(protected fiber.Router, enrolled, instructor fiber.Handler) {
	protected.Get("/courses/:course_id/smart_search", enrolled, r.SmartSearchHandler.Search)
	if r.SmartSearchHandler.Sources != nil {
		protected.Post("/courses/:course_id/smart_search/reindex", instructor, r.SmartSearchHandler.Reindex)
	}
}
