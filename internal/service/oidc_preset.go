// Package service's oidc_preset is the catalog of pre-built OIDC IdP
// configurations. Admin-UI forms read this to pre-fill issuer URLs +
// recommended scopes for the well-known providers. A "generic" preset
// always ships for catch-all use (Okta, Auth0, Keycloak, Authelia,
// Authentik, Zitadel, etc. — anything that speaks OIDC).
//
// Each preset is informational only: the admin can override every
// field. The preset name is persisted on the provider row so the
// admin UI can render a small "via Google" label without re-deriving
// it from the issuer URL.
package service

// OIDCPreset is the metadata for one provider template.
type OIDCPreset struct {
	Code         string   // matches authentication_providers.oidc_preset
	Label        string   // human-readable name
	Issuer       string   // pre-filled issuer URL; "" for generic
	Scopes       []string // recommended scopes
	Description  string   // shown in the admin UI alongside the option
	// FirstLoginOnlyClaims signals to the LoginPipeline that this IdP
	// sends email + name only on the FIRST consent (Apple's quirk).
	// The pipeline already persists outcome.Attributes into
	// federated_identities.claims_snapshot on first login; this flag
	// is informational + future-proofing.
	FirstLoginOnlyClaims bool
}

// OIDCPresets is the catalog. Order is the UI display order.
//
// Adding a preset: append here, ship a thin admin-UI render hint
// (logo + brand color). No DB change needed.
var OIDCPresets = []OIDCPreset{
	{
		Code:        "google",
		Label:       "Google Workspace",
		Issuer:      "https://accounts.google.com",
		Scopes:      []string{"openid", "email", "profile"},
		Description: "Sign in with Google Workspace for Education or a personal Google account. Free for accredited K-12 and HigherEd.",
	},
	{
		Code:        "microsoft",
		Label:       "Microsoft Entra ID",
		Issuer:      "https://login.microsoftonline.com/{tenant}/v2.0",
		Scopes:      []string{"openid", "email", "profile"},
		Description: "Sign in with Microsoft 365 / Entra ID. Replace {tenant} with your tenant id or 'organizations' for multi-tenant.",
	},
	{
		Code:                 "apple",
		Label:                "Apple Sign-In",
		Issuer:               "https://appleid.apple.com",
		Scopes:               []string{"openid", "email", "name"},
		Description:          "Sign in with Apple. Note: name + email are sent only on first consent; subsequent logins reuse the stored values.",
		FirstLoginOnlyClaims: true,
	},
	{
		Code:        "generic",
		Label:       "Generic OIDC",
		Issuer:      "",
		Scopes:      []string{"openid", "email", "profile"},
		Description: "Any OpenID Connect-compliant IdP (Okta, Auth0, Keycloak, Authelia, Authentik, Zitadel, Logto, your own). Supply issuer URL + client id/secret manually.",
	},
}

// PresetByCode returns the preset for a given code, or nil. Used by
// the admin handler when accepting form submissions to validate the
// chosen preset is known.
func PresetByCode(code string) *OIDCPreset {
	for i := range OIDCPresets {
		if OIDCPresets[i].Code == code {
			return &OIDCPresets[i]
		}
	}
	return nil
}
