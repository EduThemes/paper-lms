package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/service"
)

// registerLTIToolRoutes mounts the LTI 1.3 / Advantage endpoints that
// tools (not Paper users) call, plus the AGS / NRPS services that both
// tools and course staff use (LTI lane, G124/G127-G130/G133/G134).
//
// It MUST run before the protected group is created: Fiber mounts a
// group's middleware as a prefix-wide Use, so anything registered after
// `protected` would be forced through session auth + CSRF, which a tool
// (service token, OAuth1 or a tool-signed JWT) can never satisfy.
func (r *Router) registerLTIToolRoutes(api fiber.Router, enrolled, instructor, authLimit fiber.Handler) {
	h := r.LTIHandler
	if h == nil {
		return
	}

	// Public: the tool-signed message / registration token / OAuth1
	// signature is the credential.
	api.Get("/lti/security/openid-configuration", h.LTIOpenIDConfiguration)
	api.Post("/lti/registrations", authLimit, h.LTIDynamicRegister)
	api.Post("/lti/deep_linking_response", authLimit, h.LTIDeepLinkingResponse)
	api.Post("/lti/v1p1/tools/:tool_id/outcomes", h.LTI11Outcomes)

	// The OIDC authorization endpoint answers form_post auth requests
	// too; CSRF does not apply (see the F-055 note in router.go — the
	// session-user-match check is the defense).
	protectedOnly := r.AuthMiddleware.Protected()
	api.Post("/lti/authorize", authLimit, protectedOnly, h.LaunchDirect)
	api.Post("/lti/launch", authLimit, protectedOnly, h.LaunchDirect)

	// AGS + NRPS: a tool with a client_credentials service token (scoped,
	// installed in the course) or a course member/staff session.
	toolOrSession := h.ToolOrSessionAuth(protectedOnly)
	csrf := handlers.UnlessTool(middleware.CSRFProtection())
	audit := middleware.AuditWrites(r.AuditService, "http.write")
	ags := func(guard fiber.Handler, scopes ...string) []fiber.Handler {
		return []fiber.Handler{toolOrSession, csrf, audit, h.ToolScopeOrGuard(guard, scopes...)}
	}
	with := func(chain []fiber.Handler, final fiber.Handler) []fiber.Handler {
		return append(append([]fiber.Handler{}, chain...), final)
	}

	const base = "/lti/courses/:course_id"
	api.Get(base+"/line_items", with(ags(enrolled, service.LTIScopeLineItem, service.LTIScopeLineItemReadOnly), h.ListLineItems)...)
	api.Post(base+"/line_items", with(ags(instructor, service.LTIScopeLineItem), h.CreateLineItem)...)
	api.Get(base+"/line_items/:id", with(ags(enrolled, service.LTIScopeLineItem, service.LTIScopeLineItemReadOnly), h.GetLineItem)...)
	api.Put(base+"/line_items/:id", with(ags(instructor, service.LTIScopeLineItem), h.UpdateLineItem)...)
	api.Delete(base+"/line_items/:id", with(ags(instructor, service.LTIScopeLineItem), h.DeleteLineItem)...)
	api.Post(base+"/line_items/:id/scores", with(ags(instructor, service.LTIScopeScore), h.PostScore)...)
	api.Get(base+"/line_items/:id/results", with(ags(enrolled, service.LTIScopeResultReadOnly), h.GetResults)...)
	api.Get(base+"/memberships", with(ags(enrolled, service.LTIScopeNRPS), h.GetMemberships)...)
	api.Get(base+"/names_and_roles", with(ags(enrolled, service.LTIScopeNRPS), h.GetMemberships)...)
}

// registerLTIRoutes mounts the session-authenticated LTI routes: tool
// registration and platform keys (admin), account-level installs, tool
// placements and launches, assignment and module tool links.
func (r *Router) registerLTIRoutes(protected fiber.Router, admin, enrolled, instructor fiber.Handler) {
	h := r.LTIHandler
	if h == nil {
		return
	}

	// G123 / G134: LTI 1.3 registrations (developer key + tool configuration).
	protected.Get("/accounts/:account_id/lti_registrations", admin, h.ListLTIRegistrations)
	protected.Post("/accounts/:account_id/lti_registrations", admin, h.CreateLTIRegistration)
	protected.Post("/accounts/:account_id/lti_registrations/dynamic_registration_token", admin, h.LTIDynamicRegistrationToken)
	protected.Get("/accounts/:account_id/lti_registrations/:id", admin, h.GetLTIRegistration)
	protected.Put("/accounts/:account_id/lti_registrations/:id", admin, h.UpdateLTIRegistration)
	protected.Delete("/accounts/:account_id/lti_registrations/:id", admin, h.DeleteLTIRegistration)
	protected.Put("/accounts/:account_id/lti_registrations/:id/account_install", admin, h.SetLTIAccountInstall)
	protected.Get("/accounts/:account_id/lti_platform", admin, h.LTIPlatformInfo)

	// G126: platform key rotation is instance-wide (super admin).
	protected.Post("/lti/platform_keys/rotate", r.PermMiddleware.RequireSuperAdmin(), h.RotateLTIPlatformKey)

	// G132: tenant-wide installs.
	if r.ExternalToolHandler != nil {
		protected.Get("/accounts/:account_id/external_tools", admin, r.ExternalToolHandler.ListAccountExternalTools)
		protected.Post("/accounts/:account_id/external_tools", admin, r.ExternalToolHandler.CreateAccountExternalTool)
		protected.Delete("/accounts/:account_id/external_tools/:id", admin, r.ExternalToolHandler.DeleteAccountExternalTool)
	}

	// G131: placements, launches and tool links.
	protected.Get("/courses/:course_id/lti/tools", enrolled, h.ListCourseLTITools)
	protected.Get("/courses/:course_id/external_tools/:id/launch", enrolled, h.LaunchCourseTool)
	protected.Get("/courses/:course_id/assignments/:assignment_id/external_tool", enrolled, h.GetAssignmentExternalTool)
	protected.Put("/courses/:course_id/assignments/:assignment_id/external_tool", instructor, h.PutAssignmentExternalTool)
	protected.Delete("/courses/:course_id/assignments/:assignment_id/external_tool", instructor, h.DeleteAssignmentExternalTool)
	protected.Post("/courses/:course_id/modules/:module_id/external_tool_items", instructor, h.CreateModuleExternalToolItem)
}
