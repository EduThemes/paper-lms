package v1

import "github.com/gofiber/fiber/v2"

// registerGamificationRoutes mounts the Phase 6 / Phase 9-B / Phase
// 10-B authoring + read surfaces. The entire block is gated on
// r.GamificationHandler being non-nil, mirroring the original Register
// behavior. NOTE: the MFA self-management and passkey self-management
// routes (lines marked below) are intentionally nested inside the
// gamification gate. That coupling is historical — moving them out is
// tracked separately because doing so quietly would change behavior on
// deployments that ship without the gamification handler.
func (r *Router) registerGamificationRoutes(protected fiber.Router, admin, instructor, selfOrAdmin fiber.Handler) {
	if r.GamificationHandler == nil {
		return
	}
	gam := protected.Group("/gamification")
	gam.Get("/currencies", r.GamificationHandler.ListCurrencies)
	gam.Post("/currencies", admin, r.GamificationHandler.CreateCurrency)
	gam.Patch("/currencies/:id", admin, r.GamificationHandler.UpdateCurrency)
	gam.Delete("/currencies/:id", admin, r.GamificationHandler.DeleteCurrency)
	// selfOrAdmin populates is_admin Locals so the handler's self-or-admin
	// branch lets actual admins through. Without it, admins viewing
	// another user's wallet fall through to 403 because the handler
	// defaults is_admin=false on missing Locals.
	protected.Get("/users/:id/wallet", selfOrAdmin, r.GamificationHandler.GetUserWallet)
	protected.Get("/users/:id/wallet/transactions", selfOrAdmin, r.GamificationHandler.ListUserWalletTransactions)

	// Course-scoped instructor surface. Same handler, scope inferred
	// from :course_id presence in the URL.
	protected.Post("/courses/:course_id/gamification/currencies", instructor, r.GamificationHandler.CreateCurrency)
	protected.Patch("/courses/:course_id/gamification/currencies/:id", instructor, r.GamificationHandler.UpdateCurrency)
	protected.Delete("/courses/:course_id/gamification/currencies/:id", instructor, r.GamificationHandler.DeleteCurrency)

	// Per-learner gamification preferences (W2-C). Self-only; the
	// handler reads user_id from Locals and never accepts another
	// user's id in the path. Currently exposes the leaderboard
	// opt-out toggle.
	protected.Get("/users/self/gamification_preferences", r.GamificationHandler.GetMyGamificationPreferences)
	protected.Put("/users/self/gamification_preferences", r.GamificationHandler.UpdateMyGamificationPreferences)

	// Phase 9-B — TOTP MFA self-management (regular session required).
	// HISTORICAL NOTE: nested in the gamification gate above.
	if r.MFAHandler != nil {
		protected.Post("/users/self/mfa/enroll", r.MFAHandler.EnrollMFA)
		protected.Post("/users/self/mfa/verify-enrollment", r.MFAHandler.VerifyEnrollment)
		protected.Delete("/users/self/mfa", r.MFAHandler.DisableMFA)
	}

	// Phase 10-B — passkey self-management (regular session required).
	// Enroll/list/rename/revoke. Begin/finish login are public.
	// HISTORICAL NOTE: nested in the gamification gate above.
	if r.PasskeyHandler != nil {
		protected.Get("/users/self/passkeys", r.PasskeyHandler.List)
		protected.Post("/users/self/passkeys/begin", r.PasskeyHandler.BeginRegistration)
		protected.Post("/users/self/passkeys/finish", r.PasskeyHandler.FinishRegistration)
		protected.Patch("/users/self/passkeys/:id", r.PasskeyHandler.Rename)
		protected.Delete("/users/self/passkeys/:id", r.PasskeyHandler.Revoke)
	}

	// W2-D — Badge CRUD + per-user list + manual award/revoke.
	gam.Get("/badges", r.GamificationHandler.ListBadges)
	gam.Post("/badges", admin, r.GamificationHandler.CreateBadge)
	gam.Patch("/badges/:id", admin, r.GamificationHandler.UpdateBadge)
	gam.Delete("/badges/:id", admin, r.GamificationHandler.DeleteBadge)
	// Course-scoped instructor surface for badges. Same handler,
	// scope inferred from :course_id (resolveScope).
	protected.Post("/courses/:course_id/gamification/badges", instructor, r.GamificationHandler.CreateBadge)
	protected.Patch("/courses/:course_id/gamification/badges/:id", instructor, r.GamificationHandler.UpdateBadge)
	protected.Delete("/courses/:course_id/gamification/badges/:id", instructor, r.GamificationHandler.DeleteBadge)
	// Per-user earned-badges list. The selfOrAdmin middleware sets
	// the is_admin Locals flag the handler reads — required because
	// without it admins land in the handler's "you can only view
	// your own" branch (the handler defaults is_admin=false when
	// the Locals isn't populated by middleware).
	protected.Get("/users/:id/badges", selfOrAdmin, r.GamificationHandler.ListUserBadges)
	// Manual award / revoke. Admin only at the route level today;
	// instructor-flow lands when course-scope role check matures.
	protected.Post("/users/:user_id/badges", admin, r.GamificationHandler.AwardBadgeToUser)
	protected.Delete("/users/:user_id/badges/:badge_id", admin, r.GamificationHandler.RevokeBadgeFromUser)

	// W3-A — course leaderboard. W3-B widened access to enrolled
	// students with server-side pseudonym substitution + tenant-mode
	// render policy.
	protected.Get("/courses/:course_id/leaderboard", r.GamificationHandler.GetCourseLeaderboard)

	// W3-B — pseudonym pool discovery + learner-self switch.
	// Self-only; the handler reads user_id from Locals and never
	// accepts another user's id in the path.
	protected.Get("/courses/:course_id/gamification/pseudonym_pools", r.GamificationHandler.GetPseudonymPools)
	protected.Put("/courses/:course_id/enrollments/self/pseudonym", r.GamificationHandler.UpdatePseudonymForSelf)

	// W2-E.1 — recipe-builder write API + vocabulary discovery.
	// Vocabulary endpoint is open to any authenticated user (the
	// recipe builder UI mounts it before knowing whether the user
	// can save) — write surface is admin / instructor split exactly
	// the same way currency + badge CRUD is.
	gam.Get("/vocabulary", r.GamificationHandler.GetVocabulary)
	gam.Get("/rules", admin, r.GamificationHandler.ListRules)
	gam.Get("/rules/:id", admin, r.GamificationHandler.GetRule)
	gam.Post("/rules", admin, r.GamificationHandler.CreateRule)
	gam.Patch("/rules/:id", admin, r.GamificationHandler.PatchRule)
	gam.Delete("/rules/:id", admin, r.GamificationHandler.DeleteRule)
	// Course-scoped instructor surface for rules. Same handler,
	// scope inferred from :course_id (resolveScope).
	protected.Get("/courses/:course_id/gamification/rules", instructor, r.GamificationHandler.ListRules)
	protected.Get("/courses/:course_id/gamification/rules/:id", instructor, r.GamificationHandler.GetRule)
	protected.Post("/courses/:course_id/gamification/rules", instructor, r.GamificationHandler.CreateRule)
	protected.Patch("/courses/:course_id/gamification/rules/:id", instructor, r.GamificationHandler.PatchRule)
	protected.Delete("/courses/:course_id/gamification/rules/:id", instructor, r.GamificationHandler.DeleteRule)
}
