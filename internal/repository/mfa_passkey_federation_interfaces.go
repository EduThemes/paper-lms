package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// UserRecoveryCodeRepository (Phase 9-B) persists single-use TOTP
// recovery codes. Generated in bulk at MFA enrollment; one row
// marked used per successful recovery-code login.
type UserRecoveryCodeRepository interface {
	CreateBatch(ctx context.Context, userID uint, codeHashes []string) error
	ListUnusedForUser(ctx context.Context, userID uint) ([]models.UserRecoveryCode, error)
	MarkUsed(ctx context.Context, id uint) error
	DeleteAllForUser(ctx context.Context, userID uint) error
}

// UserWebauthnCredentialRepository (Phase 10-B) persists registered
// passkey credentials. Lookups happen on (a) credential_id for the
// assertion path and (b) user_id for the management UI. The
// assertion path also bumps SignCount + LastUsedAt on every login.
type UserWebauthnCredentialRepository interface {
	Create(ctx context.Context, cred *models.UserWebauthnCredential) error
	FindByCredentialID(ctx context.Context, credentialID []byte) (*models.UserWebauthnCredential, error)
	FindByID(ctx context.Context, id uint) (*models.UserWebauthnCredential, error)
	ListForUser(ctx context.Context, userID uint) ([]models.UserWebauthnCredential, error)
	// UpdateSignCount bumps sign_count and last_used_at after a
	// successful assertion. Replay-counter regression is the
	// library's concern, not the repo's — callers pass the verified
	// new counter through.
	UpdateSignCount(ctx context.Context, id uint, newSignCount uint32) error
	UpdateNickname(ctx context.Context, id, userID uint, nickname string) error
	Delete(ctx context.Context, id, userID uint) error
}

// FederatedIdentityRepository (Phase 9-PRE) anchors external IdP
// subjects to local user rows. Every federation handler (SAML, LDAP,
// CAS, OIDC, future WebAuthn) writes through this surface; the
// LoginPipeline reads it first when resolving an SSOOutcome to a user.
//
// Idempotent Create: re-authenticating with the same (provider,
// subject) updates last_seen_at but doesn't create a duplicate. The
// UNIQUE constraint on (provider_id, external_subject) gates it.
type FederatedIdentityRepository interface {
	// FindByProviderAndSubject returns the existing federation row or
	// (nil, nil) when no binding exists. Callers fall back to email
	// auto-link or JIT provisioning.
	FindByProviderAndSubject(ctx context.Context, providerID uint, externalSubject string) (*models.FederatedIdentity, error)
	// Create writes a fresh (user, provider, subject) binding. Caller
	// has already resolved or created the user_id.
	Create(ctx context.Context, fi *models.FederatedIdentity) error
	// TouchLastSeen bumps the last_seen_at timestamp + optionally
	// refreshes the claims_snapshot when the IdP sent richer data
	// than what was captured at first-login.
	TouchLastSeen(ctx context.Context, id uint, claimsSnapshot []byte) error
	// ListForUser is the "manage your linked accounts" view a user
	// sees in settings.
	ListForUser(ctx context.Context, userID uint) ([]models.FederatedIdentity, error)
}
