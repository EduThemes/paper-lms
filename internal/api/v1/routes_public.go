package v1

import (
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/gofiber/fiber/v2"
)

// registerPublicRoutes mounts every endpoint that does NOT require an
// authenticated session: setup wizard, login/register, public OAuth2
// token, public LTI endpoints, public SSO endpoints, OIDC client mode,
// MFA verify (pending-MFA token IS the credential), passkey
// discoverable login (the passkey IS the credential), and the public
// page endpoint.
//
// authLimit is the per-IP rate limiter middleware shared with the
// rest of the auth surface — mounted at the route level so an
// unauthenticated caller can't brute-force any single endpoint.
func (r *Router) registerPublicRoutes(api fiber.Router, authLimit fiber.Handler) {
	// Setup wizard (public, no auth required)
	api.Get("/setup/status", r.SetupHandler.GetStatus)
	api.Post("/setup/complete", middleware.AuthRateLimit(), r.SetupHandler.CompleteSetup)

	// Public auth routes (rate-limited to prevent brute-force)
	api.Post("/login", authLimit, r.UserHandler.Login)
	api.Post("/register", authLimit, r.UserHandler.Register)
	api.Post("/logout", r.UserHandler.Logout)
	api.Post("/password/reset", authLimit, r.UserHandler.RequestPasswordReset)
	api.Post("/password/reset/confirm", authLimit, r.UserHandler.ResetPassword)
	// Wave 1.6 follow-up — password-set after SIS / OneRoster
	// provisioning. Anonymous: the pending JWT (purpose=
	// password_reset_pending) IS the credential. Rate-limited like
	// the rest of the auth surface.
	api.Post("/auth/password/set", authLimit, r.UserHandler.SetPassword)

	// Public OAuth2 token endpoint (no auth required)
	api.Post("/login/oauth2/token", r.OAuth2Handler.Token)

	// Public LTI endpoints (no auth required)
	api.Get("/lti/jwks", r.LTIHandler.JWKS)
	api.Post("/lti/oidc/login", r.LTIHandler.OIDCLogin)
	api.Post("/lti/launch", r.LTIHandler.LaunchDirect)

	// Public SSO endpoints (no auth required). 13.6.B — every public
	// auth route now goes through AuthRateLimit so SAML ACS, OIDC
	// callbacks, MFA verify, and passkey begin/finish are all rate-
	// capped per IP. The limiter is still in-memory pending the Redis
	// backend (13.6.A), so multi-pod deploys split the budget; that's
	// strictly better than no limit at all.
	api.Get("/auth/saml/login", authLimit, r.SSOHandler.HandleSAMLLogin)
	api.Post("/auth/saml/acs", authLimit, r.SSOHandler.HandleSAMLACS)
	api.Get("/auth/saml/metadata", r.SSOHandler.HandleSAMLMetadata)
	api.Get("/auth/cas/login", authLimit, r.SSOHandler.HandleCASLogin)
	api.Get("/auth/cas/callback", authLimit, r.SSOHandler.HandleCASCallback)
	api.Post("/auth/ldap/login", authLimit, r.SSOHandler.HandleLDAPLogin)
	// Phase 9-A — OIDC client mode.
	if r.OIDCHandler != nil {
		api.Get("/auth/oidc/login", authLimit, r.OIDCHandler.BeginLogin)
		api.Get("/auth/oidc/callback", authLimit, r.OIDCHandler.HandleCallback)
	}
	// Phase 10-A.1 — OIDC preset catalog (public, informational).
	api.Get("/auth/oidc/presets", r.AuthProviderHandler.ListOIDCPresets)
	// Phase 9-B — TOTP 2FA step-up (no auth required; pending-MFA
	// token is the credential).
	if r.MFAHandler != nil {
		api.Post("/auth/mfa/verify", authLimit, r.MFAHandler.VerifyAtLogin)
		api.Post("/auth/mfa/recovery", authLimit, r.MFAHandler.UseRecoveryCode)
	}
	// Phase 10-B — passkey discoverable login (no auth required; the
	// passkey IS the credential). Begin/Finish ride on a 75-second
	// HttpOnly cookie carrying the encrypted ceremony state.
	if r.PasskeyHandler != nil {
		api.Post("/auth/passkey/begin", authLimit, r.PasskeyHandler.BeginLogin)
		api.Post("/auth/passkey/finish", authLimit, r.PasskeyHandler.FinishLogin)
	}

	// Public page endpoint (no auth required)
	api.Get("/courses/:course_id/p/:slug", r.PageHandler.GetPublicPage)
}
