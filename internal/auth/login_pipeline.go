// Package auth's LoginPipeline is the convergence point every credential
// type funnels through after verifying its credential.
//
// Why this exists: the pre-9-PRE codebase had four near-identical JIT
// blocks (local/saml/ldap/cas), each independently doing
// FindByLoginID → FindByEmail → Create(random_password). OIDC would
// have made it five. The audit (Sprint 7-A) flagged this as the
// load-bearing tech-debt class — fixing it here saves us from
// re-fixing it in 9-A, 9-C, and every future credential type.
//
// What the pipeline does:
//   1. Resolves the SSOOutcome to a local user row, in priority order:
//        (a) federated_identities lookup (always preferred — IdP-stable id)
//        (b) email auto-link to an existing local user (only when
//            outcome.EmailVerified is true — locked 2026-05-15)
//        (c) JIT-provision a new user (only when provider.auto_provision
//            is true)
//   2. Applies the per-tenant MFA policy from accounts.mfa_policy.
//   3. Emits either a real session JWT OR a short-lived pending-MFA
//      token, depending on (1) the user's enrollment state and (2)
//      the tenant policy.
//   4. Audit-logs every outcome (success, failure, JIT-create, link).
//
// Out of scope for the pipeline: setting cookies, formatting HTTP
// responses, talking to fiber.Ctx. The pipeline is a service; handlers
// translate its result into HTTP. This separation makes the pipeline
// fully unit-testable.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/datatypes"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// AuthProviderLookup is the narrow contract LoginPipeline needs from
// the authentication-provider surface. Declared locally so the
// pipeline package doesn't need to depend on the broader
// AuthProviderService surface. The existing postgres authProviderRepo
// satisfies it structurally — no wiring change required.
type AuthProviderLookup interface {
	FindByID(ctx context.Context, id uint) (*models.AuthenticationProvider, error)
}

// PipelineResult is what the pipeline returns. Exactly one of Token
// or PendingToken is non-empty:
//   - Token != "" → caller mints session cookies, returns dashboard
//   - PendingToken != "" → caller returns {pending_token} to the
//     client, which routes to /mfa/verify
type PipelineResult struct {
	Token        string       // full session JWT, set when no MFA gate
	PendingToken string       // short-lived JWT for /mfa/verify step
	User         *models.User // resolved user row, always set
	MustEnroll   bool         // tenant policy requires MFA but user is not enrolled
}

// LoginPipeline orchestrates post-credential-verification work for
// every login path.
type LoginPipeline struct {
	users       repository.UserRepository
	federations repository.FederatedIdentityRepository
	providers   AuthProviderLookup
	accounts    repository.AccountRepository
	audit       *AuthAudit
	jwtSecret   string
}

// AuthProviderRepository is the narrow contract LoginPipeline depends
// on from the provider surface. Defined locally because the package
// boundary (auth vs repository) doesn't yet have a typed interface
// for it — this lets pipeline tests use a minimal mock.
//
// (Future cleanup: promote this to AuthProviderLookup
// once the existing AuthProviderService is split into repo+service.)
// For now, the calling site can pass any struct that satisfies these
// two methods.

// NewLoginPipeline wires the pipeline. Caller is cmd/server/main.go.
func NewLoginPipeline(
	users repository.UserRepository,
	federations repository.FederatedIdentityRepository,
	providers AuthProviderLookup,
	accounts repository.AccountRepository,
	audit *AuthAudit,
	jwtSecret string,
) *LoginPipeline {
	return &LoginPipeline{
		users:       users,
		federations: federations,
		providers:   providers,
		accounts:    accounts,
		audit:       audit,
		jwtSecret:   jwtSecret,
	}
}

// Execute resolves an SSOOutcome to a session OR a pending-MFA token.
// The caller is responsible for setting cookies + writing the HTTP
// response; this function is HTTP-agnostic so it's directly testable.
func (p *LoginPipeline) Execute(ctx context.Context, outcome SSOOutcome, meta RequestMeta) (*PipelineResult, error) {
	// 1. Resolve user.
	user, isNew, err := p.resolveUser(ctx, outcome, meta)
	if err != nil {
		p.audit.LoginFailed(ctx, outcome.Email, err.Error(), meta)
		return nil, err
	}

	// 2. MFA policy gate.
	//
	// Sprint 10-B — passkey-as-primary: a verified WebAuthn assertion
	// proves possession of the device PLUS the device's user
	// verification (biometric/PIN). That's already two factors by
	// definition; we skip the per-tenant MFA gate entirely. The TOTP
	// step would just be friction.
	if outcome.ProviderType == "passkey" {
		token, err := GenerateToken(user, p.jwtSecret)
		if err != nil {
			return nil, err
		}
		p.audit.LoginSucceeded(ctx, user.ID, outcome.ProviderType, meta)
		_ = isNew
		return &PipelineResult{Token: token, User: user}, nil
	}

	policy := p.lookupTenantPolicy(ctx, user)
	gate := decideMFAGate(policy, user)

	switch gate {
	case mfaGateNone:
		token, err := GenerateToken(user, p.jwtSecret)
		if err != nil {
			return nil, err
		}
		p.audit.LoginSucceeded(ctx, user.ID, outcome.ProviderType, meta)
		return &PipelineResult{Token: token, User: user}, nil

	case mfaGatePending:
		pending, err := IssuePendingMFAToken(p.jwtSecret, user.ID, outcome.ProviderType)
		if err != nil {
			return nil, err
		}
		p.audit.MFARequired(ctx, user.ID, outcome.ProviderType, meta)
		return &PipelineResult{PendingToken: pending, User: user}, nil

	case mfaGateMustEnroll:
		// Policy requires MFA but user hasn't enrolled. Mint a real
		// session so they can navigate to /mfa/enroll, but flag the
		// response so the UI redirects them there immediately.
		token, err := GenerateToken(user, p.jwtSecret)
		if err != nil {
			return nil, err
		}
		p.audit.LoginSucceeded(ctx, user.ID, outcome.ProviderType, meta)
		return &PipelineResult{Token: token, User: user, MustEnroll: true}, nil
	}

	// Defensive — unreachable.
	_ = isNew
	return nil, errors.New("login pipeline: unhandled gate decision")
}

// resolveUser implements the three-step lookup: federated_identities
// → email auto-link → JIT provisioning. Returns (user, isNew, err).
func (p *LoginPipeline) resolveUser(ctx context.Context, outcome SSOOutcome, meta RequestMeta) (*models.User, bool, error) {
	// Local-password path: caller already resolved the user. We just
	// pass it through. Caller passes user_id via outcome.ExternalSubject
	// as the string form of the user id.
	//
	// Passkey path (10-B): same shape — the WebAuthn assertion
	// resolved a credential row → user_id; the handler stashes the
	// id as ExternalSubject. We skip the federated_identities lookup
	// + JIT entirely (the credential IS the binding).
	if outcome.ProviderType == "local" || outcome.ProviderType == "passkey" {
		var userID uint
		if _, err := fmt.Sscanf(outcome.ExternalSubject, "%d", &userID); err != nil || userID == 0 {
			return nil, false, errors.New("login outcome missing user id")
		}
		user, err := p.users.FindByID(ctx, userID)
		if err != nil {
			return nil, false, err
		}
		return user, false, nil
	}

	// (a) federated_identities lookup.
	if outcome.ProviderID != 0 && outcome.ExternalSubject != "" {
		fi, err := p.federations.FindByProviderAndSubject(ctx, outcome.ProviderID, outcome.ExternalSubject)
		if err != nil {
			return nil, false, err
		}
		if fi != nil {
			user, err := p.users.FindByID(ctx, fi.UserID)
			if err != nil {
				return nil, false, err
			}
			// Touch last_seen + refresh claims if richer than what
			// we have stored (Apple's first-login email quirk
			// requires keeping the original snapshot; this only
			// updates timestamps unless we have richer data).
			_ = p.federations.TouchLastSeen(ctx, fi.ID, marshalAttrsIfRicher(fi.ClaimsSnapshot, outcome.Attributes))
			return user, false, nil
		}
	}

	// (b) Email auto-link to an existing local user. Only when the
	// IdP attests email_verified=true (locked 2026-05-15).
	if outcome.EmailVerified && outcome.Email != "" {
		user, err := p.users.FindByEmail(ctx, outcome.Email)
		if err == nil && user != nil {
			// Create the federation binding so the next login uses
			// path (a) instead of re-matching by email.
			fi := &models.FederatedIdentity{
				UserID:          user.ID,
				ProviderID:      outcome.ProviderID,
				ExternalSubject: outcome.ExternalSubject,
				ClaimsSnapshot:  marshalAttrs(outcome.Attributes),
			}
			if err := p.federations.Create(ctx, fi); err != nil {
				return nil, false, fmt.Errorf("create federation binding: %w", err)
			}
			p.audit.AccountLinkedViaFederation(ctx, user.ID, outcome.ProviderType, outcome.ProviderID, meta)
			return user, false, nil
		}
	}

	// (c) JIT-provision a new user. Only when the provider's
	// auto_provision toggle is on.
	if outcome.ProviderID == 0 {
		// No provider context (shouldn't happen for non-local paths)
		// — refuse JIT.
		return nil, false, errors.New("no provider context; refusing to provision")
	}
	provider, err := p.providers.FindByID(ctx, outcome.ProviderID)
	if err != nil {
		return nil, false, err
	}
	if provider == nil || !provider.AutoProvision {
		return nil, false, fmt.Errorf("provider %d has auto-provisioning disabled; admin must pre-create the user", outcome.ProviderID)
	}

	newUser := &models.User{
		Name:         coalesce(outcome.Name, outcome.Email),
		SortableName: coalesce(outcome.Name, outcome.Email),
		ShortName:    firstWord(coalesce(outcome.Name, outcome.Email)),
		LoginID:      outcome.Email,
		Email:        outcome.Email,
		Role:         "user",
	}
	// Random placeholder password — federated users authenticate via
	// their IdP, not by knowing this. Constant-time meaningless to an
	// attacker; we just need to satisfy the NOT NULL on password_hash.
	if err := newUser.HashPassword(randomPassword()); err != nil {
		return nil, false, fmt.Errorf("hash placeholder password: %w", err)
	}
	if err := p.users.Create(ctx, newUser); err != nil {
		return nil, false, fmt.Errorf("create user: %w", err)
	}

	// Bind the federation row.
	fi := &models.FederatedIdentity{
		UserID:          newUser.ID,
		ProviderID:      outcome.ProviderID,
		ExternalSubject: outcome.ExternalSubject,
		ClaimsSnapshot:  marshalAttrs(outcome.Attributes),
	}
	if err := p.federations.Create(ctx, fi); err != nil {
		return nil, false, fmt.Errorf("create federation binding for new user: %w", err)
	}

	p.audit.UserProvisionedViaJIT(ctx, newUser.ID, outcome.ProviderType, outcome.ProviderID, meta)
	return newUser, true, nil
}

// MFA gate decision values. Internal — handler reads PipelineResult.
type mfaGate int

const (
	mfaGateNone       mfaGate = iota // mint normal session
	mfaGatePending                   // mint pending-MFA token; user must verify
	mfaGateMustEnroll                // mint normal session + flag "must enroll"
)

func decideMFAGate(policy string, user *models.User) mfaGate {
	enrolled := user.TOTPVerifiedAt != nil
	switch policy {
	case "off":
		return mfaGateNone
	case "optional":
		if enrolled {
			return mfaGatePending
		}
		return mfaGateNone
	case "required_admin":
		// Admins must enroll AND verify. Non-admins follow optional.
		if user.Role == "admin" {
			if !enrolled {
				return mfaGateMustEnroll
			}
			return mfaGatePending
		}
		if enrolled {
			return mfaGatePending
		}
		return mfaGateNone
	case "required_all":
		if !enrolled {
			return mfaGateMustEnroll
		}
		return mfaGatePending
	}
	// Unknown policy → safest is "off". Admin misconfigurations
	// shouldn't lock everyone out.
	return mfaGateNone
}

// lookupTenantPolicy reads accounts.mfa_policy for the user's home
// account. v1: always account_id=1 (single-tenant). Multi-tenant
// resolution is a Phase 10 concern.
func (p *LoginPipeline) lookupTenantPolicy(ctx context.Context, user *models.User) string {
	if p.accounts == nil {
		return "off"
	}
	acc, err := p.accounts.FindByID(ctx, 1)
	if err != nil || acc == nil {
		return "off"
	}
	return acc.MFAPolicy
}

// ----- small helpers -----

func coalesce(s, fallback string) string {
	if strings.TrimSpace(s) != "" {
		return s
	}
	return fallback
}

func firstWord(s string) string {
	for _, w := range strings.Fields(s) {
		return w
	}
	return s
}

// randomPassword generates a placeholder password for JIT-provisioned
// federated users. They authenticate via their IdP, not this string;
// it exists only to satisfy the NOT NULL constraint on password_hash.
func randomPassword() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func marshalAttrs(attrs map[string]any) datatypes.JSON {
	if len(attrs) == 0 {
		return nil
	}
	// json.Marshal of a map[string]any is stable enough for storage;
	// we don't sort because the snapshot is informational.
	b, err := json.Marshal(attrs)
	if err != nil {
		return nil
	}
	return datatypes.JSON(b)
}

// marshalAttrsIfRicher returns a fresh snapshot only when the new
// attributes contain a key the existing snapshot doesn't. Apple's
// first-login email is the canonical example: subsequent logins omit
// it, but we never want to overwrite the captured email with nothing.
func marshalAttrsIfRicher(existing datatypes.JSON, attrs map[string]any) []byte {
	if len(attrs) == 0 {
		return nil
	}
	if len(existing) == 0 {
		b, _ := json.Marshal(attrs)
		return b
	}
	// For v1 we never overwrite. Future enhancement: merge in NEW
	// keys but never replace existing ones.
	return nil
}
