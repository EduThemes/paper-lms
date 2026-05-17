package v1

import (
	"github.com/gofiber/fiber/v2"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
)

// registerSuperAdminRoutes mounts every route gated by the
// super_admin role. Wave 2 ships the read API; Wave 3 adds the write
// API + four test actions; Wave 5 will refactor env-var consumers to
// read through the settings service.
//
// SECURITY CONTRACT
// ─────────────────
// This function is the ONLY authorized place to mount /superadmin/*
// routes. Every route on the returned sub-group automatically inherits
// Protected() (from the parent group) AND RequireSuperAdmin() (from
// the sub-group below). Forgetting either is a deployment-wide
// privilege escalation — see the Canvas-LMS site_admin CVEs
// (e.g. CVE-2021-32585). The router wrapper here is the structural
// guarantee against that footgun.
//
// DO NOT mount super-admin routes on `protected` directly; the
// sub-group below is what carries the role check. If you need a new
// gate variant (e.g. RequireSuperAdminWithMFAStepUp), add it here as
// a chained handler — never bypass the existing chain.
func (r *Router) registerSuperAdminRoutes(protected fiber.Router) {
	superAdmin := protected.Group(
		"/superadmin",
		r.PermMiddleware.RequireSuperAdmin(),
	)

	h := r.SuperAdminSettingsHandler

	// ── Read API (Wave 2) ──
	// IMPORTANT: register /settings/groups BEFORE /settings/:key so
	// the literal path wins over the wildcard. Same pattern as
	// /commons/favorites in routes_p3_features.go.
	superAdmin.Get("/settings/groups", h.Groups)
	superAdmin.Get("/settings", h.List)

	// ── Test actions (Wave 3) ──
	// Each test endpoint sits behind a per-action rate limit
	// (1 request per 30s per (super-admin, action)) to block loop-
	// testing. The /test/* literal-prefix routes MUST be declared
	// before /settings/:key so they aren't shadowed by the wildcard.
	superAdmin.Post("/settings/test/email",
		middleware.SuperAdminTestRateLimit("email"), h.TestEmail)
	superAdmin.Post("/settings/test/oidc",
		middleware.SuperAdminTestRateLimit("oidc"), h.TestOIDC)
	superAdmin.Post("/settings/test/anthropic",
		middleware.SuperAdminTestRateLimit("anthropic"), h.TestAnthropic)
	superAdmin.Post("/settings/test/s3",
		middleware.SuperAdminTestRateLimit("s3"), h.TestS3)

	// ── Single-key read + write (Wave 2 + Wave 3) ──
	// :key is the most permissive matcher; declare last so the
	// literal-path test/groups/etc. routes above win.
	superAdmin.Get("/settings/:key", h.Get)
	superAdmin.Put("/settings/:key", h.Set)
	superAdmin.Delete("/settings/:key", h.Clear)
}
