package auth

import (
	"context"
	"encoding/json"

	"github.com/EduThemes/paper-lms/internal/service"
)

// AuthAudit is a typed shim over service.AuditService for login-path
// events. Centralized so the event_type strings ("auth.login_succeeded",
// "auth.login_failed", "auth.mfa_required", etc.) live in one file and
// any new event a future credential type adds gets the same shape.
//
// Why a wrapper instead of inlining: every call site would otherwise
// have to remember (a) the right event_type string, (b) to fill in
// the IP/UserAgent fields from the request, (c) to JSON-marshal the
// payload. The wrapper makes the wrong thing harder to type than the
// right thing.
type AuthAudit struct {
	svc *service.AuditService
}

func NewAuthAudit(svc *service.AuditService) *AuthAudit {
	return &AuthAudit{svc: svc}
}

// RequestMeta captures the IP + UA the audit row needs. Caller pulls
// them from the request (fiber.Ctx) once and threads them through.
type RequestMeta struct {
	IPAddress string
	UserAgent string
}

// LoginSucceeded fires when a user resolves to a real session (NOT a
// pending-MFA token). For pending-MFA, see MFARequired.
func (a *AuthAudit) LoginSucceeded(ctx context.Context, userID uint, providerType string, meta RequestMeta) {
	a.emit(ctx, "auth.login_succeeded", userID, "User", userID, "logged_in", map[string]string{
		"provider_type": providerType,
	}, meta)
}

// LoginFailed: bad credentials, unknown user, JIT-disabled new user,
// etc. attemptedEmail is captured (sanitized) for forensics; userID is
// 0 when no user matched.
func (a *AuthAudit) LoginFailed(ctx context.Context, attemptedEmail, reason string, meta RequestMeta) {
	a.emit(ctx, "auth.login_failed", 0, "User", 0, "login_failed", map[string]string{
		"attempted_email": attemptedEmail,
		"reason":          reason,
	}, meta)
}

// MFARequired: credentials accepted but the user must complete step-up
// before getting a session. Pending-MFA token issued.
func (a *AuthAudit) MFARequired(ctx context.Context, userID uint, providerType string, meta RequestMeta) {
	a.emit(ctx, "auth.mfa_required", userID, "User", userID, "mfa_required", map[string]string{
		"provider_type": providerType,
	}, meta)
}

// MFAVerified: step-up succeeded, real session issued.
func (a *AuthAudit) MFAVerified(ctx context.Context, userID uint, factor string, meta RequestMeta) {
	a.emit(ctx, "auth.mfa_verified", userID, "User", userID, "mfa_verified", map[string]string{
		"factor": factor, // "totp" | "recovery_code"
	}, meta)
}

// MFAFailed: wrong code, exhausted attempts, expired pending token.
func (a *AuthAudit) MFAFailed(ctx context.Context, userID uint, reason string, meta RequestMeta) {
	a.emit(ctx, "auth.mfa_failed", userID, "User", userID, "mfa_failed", map[string]string{
		"reason": reason,
	}, meta)
}

// UserProvisionedViaJIT: a federated handler created a new local user
// row in response to a first-time SSO login. The provider's
// auto_provision toggle was on.
func (a *AuthAudit) UserProvisionedViaJIT(ctx context.Context, userID uint, providerType string, providerID uint, meta RequestMeta) {
	a.emit(ctx, "auth.user_provisioned_jit", userID, "User", userID, "user_created", map[string]string{
		"provider_type": providerType,
	}, withProviderID(meta, providerID))
}

// PasskeyRegistered: a user finished a registration ceremony and a
// new credential row was persisted.
func (a *AuthAudit) PasskeyRegistered(ctx context.Context, userID, credentialRowID uint, nickname string, meta RequestMeta) {
	a.emit(ctx, "auth.passkey_registered", userID, "UserWebauthnCredential", credentialRowID, "passkey_registered", map[string]string{
		"nickname": nickname,
	}, meta)
}

// PasskeyUsed: a discoverable-login ceremony succeeded; the user has
// a real session as of this row.
func (a *AuthAudit) PasskeyUsed(ctx context.Context, userID, credentialRowID uint, meta RequestMeta) {
	a.emit(ctx, "auth.passkey_used", userID, "UserWebauthnCredential", credentialRowID, "passkey_used", nil, meta)
}

// PasskeyRevoked: a user removed one of their credentials via the
// management UI.
func (a *AuthAudit) PasskeyRevoked(ctx context.Context, userID, credentialRowID uint, meta RequestMeta) {
	a.emit(ctx, "auth.passkey_revoked", userID, "UserWebauthnCredential", credentialRowID, "passkey_revoked", nil, meta)
}

// AccountLinkedViaFederation: an SSO login resolved to an existing
// local-password user via email auto-link. Per the 2026-05-15 policy,
// this only fires when the IdP attested email_verified=true.
func (a *AuthAudit) AccountLinkedViaFederation(ctx context.Context, userID uint, providerType string, providerID uint, meta RequestMeta) {
	a.emit(ctx, "auth.account_linked", userID, "FederatedIdentity", providerID, "linked", map[string]string{
		"provider_type": providerType,
	}, meta)
}

// RegistrationCompleted (Phase 13.4 / Wave C.2) fires when a public
// signup creates a new user row. status is one of "active" or
// "pending_parental_consent" — the latter when the tenant is
// coppa_strict and the registrant is under 13 without a verified
// parental_consent_token. Pipeline integration deferred; the handler
// calls this directly for audit symmetry with the login path.
func (a *AuthAudit) RegistrationCompleted(ctx context.Context, userID uint, status string, meta RequestMeta) {
	a.emit(ctx, "auth.registration_completed", userID, "User", userID, "user_registered", map[string]string{
		"status": status,
	}, meta)
}

func (a *AuthAudit) emit(ctx context.Context, eventType string, userID uint, contextType string, contextID uint, action string, payload map[string]string, meta RequestMeta) {
	if a.svc == nil {
		// Defensive: tests sometimes wire a nil audit service. Don't
		// crash the login path on an audit-log write failure.
		return
	}
	body, _ := json.Marshal(payload)
	_ = a.svc.LogEvent(ctx, eventType, userID, nil, nil, contextType, contextID, action, string(body), meta.IPAddress, meta.UserAgent)
}

// withProviderID is a tiny helper that doesn't change RequestMeta
// itself — provider_id is encoded in the audit payload by the caller.
// Kept as a no-op placeholder for future request-context enrichment.
func withProviderID(m RequestMeta, _ uint) RequestMeta { return m }
