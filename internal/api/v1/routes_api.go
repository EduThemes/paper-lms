package v1

import "github.com/gofiber/fiber/v2"

// useCanvasIDs mounts the Canvas id resolver (G137: "self" and
// sis_*_id: path segments) on the protected group. It must run after
// authentication and before any protected route is registered, so it is
// called right after the group's other Use middlewares in Register.
func (r *Router) useCanvasIDs(protected fiber.Router) {
	if r.CanvasIDs != nil {
		protected.Use(r.CanvasIDs)
	}
}

// registerAPICompatRoutes mounts the Canvas REST API compatibility surface
// (the "api" parity lane):
//
//   - G136: the OAuth2 provider at Canvas's root paths (/login/oauth2/*),
//     token DELETE (logout) and /login/session_token;
//   - G138: course rosters, account course/user lists, the multi-student
//     submissions feed and account reports;
//   - G143: personal access token show / update / regenerate and the
//     user_generated_tokens list.
func (r *Router) registerAPICompatRoutes(app *fiber.App, protected fiber.Router, authLimit, admin, enrolled, selfOrAdmin fiber.Handler) {
	// --- G136: Canvas clients hard-code the OAuth endpoints at the site
	// root, not under /api/v1. The /api/v1 copies stay for existing users.
	app.Get("/login/oauth2/auth", authLimit, r.OAuth2Handler.Authorize)
	app.Post("/login/oauth2/token", r.OAuth2Handler.Token)
	app.Delete("/login/oauth2/token", r.AuthMiddleware.Protected(), r.OAuth2Handler.Logout)
	if r.LTIHandler != nil {
		app.Get("/login/oauth2/jwks", r.LTIHandler.JWKS)
	}
	protected.Delete("/login/oauth2/token", r.OAuth2Handler.Logout)
	if r.SessionTokenHandler != nil {
		// ?session_token=... redeems (public); without it, an API caller
		// mints a session_url.
		app.Get("/login/session_token", authLimit, r.SessionTokenHandler.Redeem, r.AuthMiddleware.Protected(), r.SessionTokenHandler.Mint)
		protected.Get("/login/session_token", r.SessionTokenHandler.Mint)
	}

	// --- G138: Canvas collection endpoints scripts depend on.
	if r.CanvasAPIHandler != nil {
		protected.Get("/courses/:course_id/users", enrolled, r.CanvasAPIHandler.ListCourseUsers)
		protected.Get("/courses/:course_id/search_users", enrolled, r.CanvasAPIHandler.ListCourseUsers)
		protected.Get("/courses/:course_id/students/submissions", enrolled, r.CanvasAPIHandler.ListStudentSubmissions)
		protected.Get("/accounts/:account_id/courses", admin, r.CanvasAPIHandler.ListAccountCourses)
		protected.Get("/accounts/:account_id/users", admin, r.CanvasAPIHandler.ListAccountUsers)
	}
	// Account reports (G138) are registered with the account admin routes
	// (routes_admin_accounts.go, one implementation for G117 and G138).

	// --- G143: personal access token lifecycle.
	protected.Get("/users/:user_id/user_generated_tokens", selfOrAdmin, r.AccessTokenHandler.ListUserGeneratedTokens)
	protected.Get("/users/:user_id/tokens/:id", selfOrAdmin, r.AccessTokenHandler.GetAccessToken)
	protected.Put("/users/:user_id/tokens/:id", selfOrAdmin, r.AccessTokenHandler.UpdateAccessToken)
}
