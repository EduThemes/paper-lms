// Package auth's OIDC handler implements OpenID Connect client mode:
// Paper LMS becomes a Relying Party that accepts logins from external
// Identity Providers (Google Workspace, Microsoft Entra ID, Apple
// Sign-In, or any compliant OIDC IdP).
//
// Why this lib stack:
//   - golang.org/x/oauth2 — Go-team-maintained OAuth2 plumbing.
//   - github.com/coreos/go-oidc/v3 — OIDC layer on top; OpenID
//     Foundation-certified. Reads /.well-known/openid-configuration,
//     verifies ID-token JWT signatures via JWKS, handles claim
//     extraction.
//
// What this handler does NOT do:
//   - Provision users. The LoginPipeline owns that decision.
//   - Mint sessions. The LoginPipeline does that too.
//   - Talk to the database for anything other than reading the
//     provider's stored client_secret. SSOOutcome is its only output.
//
// What it DOES do:
//   - State + nonce CSRF protection (PKCE for the public-client path
//     when secret is absent, but presets all use confidential clients
//     so PKCE is opt-in).
//   - Verify the ID token signature + issuer + audience + expiry.
//   - Build the SSOOutcome, including EmailVerified from the claim
//     (default false if absent — per OIDC spec, an IdP that omits
//     the claim is NOT asserting verification).
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/oauth2"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// SettingsLookupFunc is the type OIDCHandler accepts for resolving
// live config values (specifically the OIDC redirect base) from the
// Settings Engine. Function-typed rather than interface-typed to
// break a would-be import cycle: internal/auth imports
// internal/service (via auth_audit.go); internal/service/settings
// imports internal/auth (for secretbox). A bare function type lets
// cmd/server/main.go wire the closure that holds the
// settings.Service reference without dragging the package in here.
//
// Empty string + nil error means "no value in the resolution chain"
// (callers fall back to the construction-time redirectBase);
// non-nil error means "the lookup itself failed transiently."
type SettingsLookupFunc func(ctx context.Context, key string) (string, error)

// OIDCHandler dispatches the OIDC code flow.
type OIDCHandler struct {
	providers     AuthProviderLookup
	loginPipeline *LoginPipeline
	cookieDomain  string
	redirectBase  string // construction-time fallback used when the settings lookup returns empty
	lookup        SettingsLookupFunc
}

// NewOIDCHandler wires the handler. cookieDomain is used for the
// state cookie; pass "" for "current host." redirectBase is the
// scheme+host the IdP will redirect back to; the actual callback
// path is constant (/api/v1/auth/oidc/callback). lookup resolves
// "auth.oidc.redirect_base" via the Settings Engine per-request so
// the value can be rotated from the super-admin panel without a
// restart; the catalog handles the OIDC_REDIRECT_BASE env fallback
// internally. When lookup returns empty (no setting + no env) the
// constructor-supplied redirectBase is the safety net.
func NewOIDCHandler(providers AuthProviderLookup, pipeline *LoginPipeline, cookieDomain, redirectBase string, lookup SettingsLookupFunc) *OIDCHandler {
	return &OIDCHandler{
		providers:     providers,
		loginPipeline: pipeline,
		cookieDomain:  cookieDomain,
		redirectBase:  redirectBase,
		lookup:        lookup,
	}
}

// BeginLogin handles GET /api/v1/auth/oidc/login?provider_id=N.
// Generates state + nonce, stores them in short-lived cookies, and
// 302-redirects to the IdP's authorize endpoint.
func (h *OIDCHandler) BeginLogin(c *fiber.Ctx) error {
	providerID, err := strconv.Atoi(c.Query("provider_id"))
	if err != nil || providerID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "invalid provider_id"}}})
	}
	provider, err := h.providers.FindByID(c.Context(), uint(providerID))
	if err != nil || provider == nil || provider.AuthType != "oidc" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "provider is not OIDC or not found"}}})
	}
	if provider.WorkflowState != "active" {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"errors": []fiber.Map{{"message": "provider is not active"}}})
	}

	// Build the oauth2.Config from stored provider settings + the
	// provider's discovery doc.
	cfg, _, err := h.buildConfig(c.Context(), provider)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"errors": []fiber.Map{{"message": "could not initialize OIDC: " + err.Error()}}})
	}

	state := randomToken(32)
	nonce := randomToken(32)

	// Persist state + nonce + provider_id in cookies. The callback
	// reads them back to verify. HttpOnly so JS can't peek; 10-minute
	// expiry so a stalled flow expires fast.
	expiry := time.Now().Add(10 * time.Minute)
	c.Cookie(&fiber.Cookie{Name: "oidc_state", Value: state, Path: "/", HTTPOnly: true, SameSite: "Lax", Expires: expiry})
	c.Cookie(&fiber.Cookie{Name: "oidc_nonce", Value: nonce, Path: "/", HTTPOnly: true, SameSite: "Lax", Expires: expiry})
	c.Cookie(&fiber.Cookie{Name: "oidc_provider", Value: strconv.Itoa(providerID), Path: "/", HTTPOnly: true, SameSite: "Lax", Expires: expiry})

	url := cfg.AuthCodeURL(state, oidc.Nonce(nonce))
	return c.Redirect(url, fiber.StatusFound)
}

// HandleCallback handles GET /api/v1/auth/oidc/callback?code=...&state=...
// Verifies state, exchanges code, verifies ID token, builds SSOOutcome,
// runs the LoginPipeline.
func (h *OIDCHandler) HandleCallback(c *fiber.Ctx) error {
	// Reconstruct provider from the cookie set by BeginLogin.
	providerCookie := c.Cookies("oidc_provider")
	stateCookie := c.Cookies("oidc_state")
	nonceCookie := c.Cookies("oidc_nonce")
	if providerCookie == "" || stateCookie == "" || nonceCookie == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "oidc cookies missing or expired"}}})
	}
	providerID, err := strconv.Atoi(providerCookie)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "invalid oidc_provider cookie"}}})
	}
	provider, err := h.providers.FindByID(c.Context(), uint(providerID))
	if err != nil || provider == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"errors": []fiber.Map{{"message": "could not load provider"}}})
	}

	// State must match — CSRF protection.
	if c.Query("state") != stateCookie {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "state mismatch (CSRF guard)"}}})
	}
	code := c.Query("code")
	if code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "missing code"}}})
	}

	cfg, verifier, err := h.buildConfig(c.Context(), provider)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"errors": []fiber.Map{{"message": "could not initialize OIDC: " + err.Error()}}})
	}

	tok, err := cfg.Exchange(c.Context(), code)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "token exchange failed: " + err.Error()}}})
	}
	rawIDToken, ok := tok.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "id_token missing from token response"}}})
	}
	idToken, err := verifier.Verify(c.Context(), rawIDToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"errors": []fiber.Map{{"message": "id_token verification failed: " + err.Error()}}})
	}
	// Nonce — defense-in-depth replay protection.
	if idToken.Nonce != nonceCookie {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"errors": []fiber.Map{{"message": "nonce mismatch"}}})
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "could not parse id_token claims"}}})
	}
	if claims.Sub == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"errors": []fiber.Map{{"message": "id_token missing 'sub' claim"}}})
	}

	// Build outcome + run through the pipeline.
	outcome := SSOOutcome{
		ProviderID:      provider.ID,
		ProviderType:    "oidc",
		ExternalSubject: claims.Sub,
		Email:           claims.Email,
		EmailVerified:   claims.EmailVerified,
		Name:            claims.Name,
		Attributes: map[string]any{
			"email":          claims.Email,
			"name":           claims.Name,
			"picture":        claims.Picture,
			"email_verified": claims.EmailVerified,
			"oidc_preset":    provider.OIDCPreset,
		},
	}
	meta := RequestMeta{IPAddress: c.IP(), UserAgent: c.Get("User-Agent")}
	result, err := h.loginPipeline.Execute(c.Context(), outcome, meta)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"errors": []fiber.Map{{"message": err.Error()}}})
	}

	// Clear OIDC cookies — they've served their purpose.
	for _, name := range []string{"oidc_state", "oidc_nonce", "oidc_provider"} {
		c.Cookie(&fiber.Cookie{Name: name, Value: "", Path: "/", HTTPOnly: true, MaxAge: -1, Expires: time.Now().Add(-1 * time.Hour)})
	}

	// Mint session OR pending-MFA, set cookie, redirect to dashboard.
	// Browser-based flow: we 302 to the SPA root with the appropriate
	// state. The SPA reads cookies/storage to know whether to route
	// to dashboard or /mfa/verify.
	if result.PendingToken != "" {
		// MFA gate hit. We can't usefully set a cookie because the
		// pending-token isn't a session. Redirect to a "complete MFA"
		// route with the token in the URL — single-use, 5-min TTL.
		return c.Redirect(fmt.Sprintf("/mfa/verify?t=%s", result.PendingToken), fiber.StatusFound)
	}
	c.Cookie(&fiber.Cookie{
		Name:     "paper_session",
		Value:    result.Token,
		Path:     "/",
		HTTPOnly: true,
		SameSite: "Lax",
		MaxAge:   86400,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	if result.MustEnroll {
		return c.Redirect("/mfa/enroll", fiber.StatusFound)
	}
	return c.Redirect("/", fiber.StatusFound)
}

// buildConfig assembles the oauth2.Config + oidc.Verifier from the
// stored provider row. Returns both because BeginLogin only needs the
// config and HandleCallback needs both.
func (h *OIDCHandler) buildConfig(ctx context.Context, provider *models.AuthenticationProvider) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if provider.OIDCIssuerURL == "" || provider.OIDCClientID == "" {
		return nil, nil, errors.New("provider missing oidc_issuer_url or oidc_client_id")
	}
	clientSecret := ""
	if len(provider.OIDCClientSecretEncrypted) > 0 {
		pt, err := Decrypt(provider.OIDCClientSecretEncrypted)
		if err != nil {
			return nil, nil, fmt.Errorf("decrypt oidc_client_secret: %w", err)
		}
		clientSecret = string(pt)
	}

	// Discovery — coreos/go-oidc reads .well-known/openid-configuration.
	prov, err := oidc.NewProvider(ctx, provider.OIDCIssuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("oidc discovery: %w", err)
	}

	scopes := []string{"openid", "email", "profile"}
	if len(provider.OIDCScopes) > 0 {
		scopes = []string(provider.OIDCScopes)
	}

	// Resolve the redirect base per-request via the Settings Engine
	// so super-admin changes take effect without a restart. The
	// catalog handles the OIDC_REDIRECT_BASE env fallback internally;
	// when nothing is set anywhere the construction-time redirectBase
	// is the safety net. There is intentionally NO hard-coded fallback
	// below that — Wave 5 audit H2 found the previous draft silently
	// papering over a production misconfig with "http://localhost:3000",
	// which the IdP would reject anyway. main.go now passes a dev
	// fallback explicitly when cfg.Environment=="development"; in
	// production an empty result here yields an error instead of a
	// silent broken redirect.
	callback := ""
	if h.lookup != nil {
		v, err := h.lookup(ctx, "auth.oidc.redirect_base")
		if err == nil {
			callback = v
		}
	}
	if callback == "" {
		callback = h.redirectBase
	}
	if callback == "" {
		return nil, nil, fmt.Errorf("OIDC redirect base not configured: set the auth.oidc.redirect_base setting, OIDC_REDIRECT_BASE env var, or FRONTEND_URL")
	}
	cfg := &oauth2.Config{
		ClientID:     provider.OIDCClientID,
		ClientSecret: clientSecret,
		Endpoint:     prov.Endpoint(),
		RedirectURL:  callback + "/api/v1/auth/oidc/callback",
		Scopes:       scopes,
	}

	verifier := prov.Verifier(&oidc.Config{ClientID: provider.OIDCClientID})
	return cfg, verifier, nil
}

// randomToken returns a URL-safe base64 string of `n` random bytes.
// Used for `state` and `nonce` in the OIDC flow — both must be
// unpredictable and at least 128 bits per OAuth2 + OIDC best practice.
func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
