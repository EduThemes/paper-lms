package auth

// SSOOutcome is the shape every credential-verification handler
// produces and the LoginPipeline consumes. Convergence point for
// local-password, SAML, LDAP, CAS, OIDC, and (later) WebAuthn.
//
// Why a struct instead of separate function signatures: the pipeline's
// JIT-provisioning + auto-link + MFA-gate logic is identical across
// credential types. Funneling them all through one shape eliminates
// the W2-D-style class of bug where SAML's JIT diverges from LDAP's
// JIT diverges from CAS's JIT. New credential types add a producer,
// not a third copy of the logic.
//
// ProviderType values: "local" | "saml" | "ldap" | "cas" | "oidc" |
// (future) "passkey". The pipeline branches on this only for:
//   - emitting audit events with the right label
//   - skipping the `federated_identities` lookup for "local"
//
// ExternalSubject is the IdP-stable identifier:
//   - SAML: NameID
//   - OIDC: sub claim
//   - LDAP: full DN
//   - CAS: principal name
//   - local: "" (unused — local auth resolves by login_id)
//
// EmailVerified is the trust signal for auto-linking to an existing
// local-password user with the same email. Per user decision
// 2026-05-15: link only when this is true. Per-protocol semantics:
//   - SAML: true (IdP attests via signed assertion)
//   - LDAP: true (the directory authenticated)
//   - CAS: true (ticket validated)
//   - OIDC: respect the IdP's email_verified claim; default false if absent
//   - local: true (the user authenticated against their own row)
//
// Attributes carries the raw claims/attributes for audit and future
// JIT-mapping use. The pipeline persists a subset into
// federated_identities.claims_snapshot.
type SSOOutcome struct {
	ProviderID      uint
	ProviderType    string
	ExternalSubject string
	Email           string
	EmailVerified   bool
	Name            string
	Attributes      map[string]any
}
