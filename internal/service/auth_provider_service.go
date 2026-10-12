package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LDAPTestResult holds the result of an LDAP connection test.
type LDAPTestResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// AuthProviderService manages authentication provider configurations.
type AuthProviderService struct {
	repo repository.AuthenticationProviderRepository
}

// NewAuthProviderService creates a new AuthProviderService.
func NewAuthProviderService(repo repository.AuthenticationProviderRepository) *AuthProviderService {
	return &AuthProviderService{repo: repo}
}

// CreateProvider validates the auth_type and creates a new authentication provider.
func (s *AuthProviderService) CreateProvider(ctx context.Context, provider *models.AuthenticationProvider) error {
	if !isValidAuthType(provider.AuthType) {
		return errors.New("auth_type must be one of: saml, ldap, cas, oidc")
	}

	if provider.WorkflowState == "" {
		provider.WorkflowState = "active"
	}

	return s.repo.Create(ctx, provider)
}

// ErrAuthProviderNotFound is returned when a provider does not exist or
// lives in a different account than the one the caller is scoped to.
// The two cases are deliberately indistinguishable (existence-leak
// contract: cross-tenant reads 404, never 403).
var ErrAuthProviderNotFound = errors.New("authentication provider not found")

// findScoped loads a provider by id and ties it to accountID. Every
// by-id operation goes through here so an admin of one tenant can never
// read, rewrite or delete another tenant's SSO configuration.
// accountID==0 is the auth-internal escape hatch (no tenant filter);
// the admin API never passes 0.
func (s *AuthProviderService) findScoped(ctx context.Context, id, accountID uint) (*models.AuthenticationProvider, error) {
	provider, err := s.repo.FindByID(ctx, id)
	if err != nil || provider == nil {
		return nil, ErrAuthProviderNotFound
	}
	if accountID != 0 && provider.AccountID != accountID {
		return nil, ErrAuthProviderNotFound
	}
	return provider, nil
}

// GetProvider retrieves an authentication provider by ID within accountID.
func (s *AuthProviderService) GetProvider(ctx context.Context, id, accountID uint) (*models.AuthenticationProvider, error) {
	return s.findScoped(ctx, id, accountID)
}

// UpdateProvider finds an existing provider by ID (within accountID),
// applies updates, and saves.
func (s *AuthProviderService) UpdateProvider(ctx context.Context, id, accountID uint, updates *models.AuthenticationProvider) (*models.AuthenticationProvider, error) {
	existing, err := s.findScoped(ctx, id, accountID)
	if err != nil {
		return nil, err
	}

	// Apply updates selectively
	if updates.AuthType != "" {
		if !isValidAuthType(updates.AuthType) {
			return nil, errors.New("auth_type must be one of: saml, ldap, cas")
		}
		existing.AuthType = updates.AuthType
	}
	if updates.Position != 0 {
		existing.Position = updates.Position
	}

	// SAML fields
	if updates.IDPEntityID != "" {
		existing.IDPEntityID = updates.IDPEntityID
	}
	if updates.LogInURL != "" {
		existing.LogInURL = updates.LogInURL
	}
	if updates.LogOutURL != "" {
		existing.LogOutURL = updates.LogOutURL
	}
	if updates.CertificateFingerprint != "" {
		existing.CertificateFingerprint = updates.CertificateFingerprint
	}

	// LDAP fields
	if updates.LDAPHost != "" {
		existing.LDAPHost = updates.LDAPHost
	}
	if updates.LDAPPort != 0 {
		existing.LDAPPort = updates.LDAPPort
	}
	if updates.LDAPBase != "" {
		existing.LDAPBase = updates.LDAPBase
	}
	if updates.LDAPFilter != "" {
		existing.LDAPFilter = updates.LDAPFilter
	}
	if updates.LDAPBindDN != "" {
		existing.LDAPBindDN = updates.LDAPBindDN
	}
	if updates.LDAPBindPassword != "" {
		existing.LDAPBindPassword = updates.LDAPBindPassword
	}
	// Mirror of the OIDC ciphertext-rotation pattern below: a zero-length
	// LDAPBindPasswordEncrypted means "admin didn't rotate the bind
	// password" — keep whatever ciphertext is on disk. Any non-empty
	// value is a fresh secretbox-encrypted rotation from the handler.
	if len(updates.LDAPBindPasswordEncrypted) > 0 {
		existing.LDAPBindPasswordEncrypted = updates.LDAPBindPasswordEncrypted
		// A new encrypted secret is the canonical write — make sure no
		// stale plaintext can shadow it once Wave-B drops the column.
		existing.LDAPBindPassword = ""
	}
	if updates.LDAPLoginAttribute != "" {
		existing.LDAPLoginAttribute = updates.LDAPLoginAttribute
	}
	// Boolean fields — always apply from updates struct
	existing.LDAPUseTLS = updates.LDAPUseTLS
	existing.JITProvisioning = updates.JITProvisioning

	// CAS fields
	if updates.CASBaseURL != "" {
		existing.CASBaseURL = updates.CASBaseURL
	}
	if updates.CASLoginURL != "" {
		existing.CASLoginURL = updates.CASLoginURL
	}
	if updates.CASValidateURL != "" {
		existing.CASValidateURL = updates.CASValidateURL
	}
	if updates.CASLogoutURL != "" {
		existing.CASLogoutURL = updates.CASLogoutURL
	}

	// General settings
	if updates.FederatedAttributes != nil {
		existing.FederatedAttributes = updates.FederatedAttributes
	}
	if updates.WorkflowState != "" {
		existing.WorkflowState = updates.WorkflowState
	}

	// OIDC fields (Phase 10-A.1). Selective: empty strings leave
	// existing values intact; an empty ClientSecretEncrypted means
	// "admin didn't rotate the secret" — keep the stored ciphertext.
	if updates.OIDCIssuerURL != "" {
		existing.OIDCIssuerURL = updates.OIDCIssuerURL
	}
	if updates.OIDCClientID != "" {
		existing.OIDCClientID = updates.OIDCClientID
	}
	if len(updates.OIDCClientSecretEncrypted) > 0 {
		existing.OIDCClientSecretEncrypted = updates.OIDCClientSecretEncrypted
	}
	if len(updates.OIDCScopes) > 0 {
		existing.OIDCScopes = updates.OIDCScopes
	}
	if updates.OIDCPreset != "" {
		existing.OIDCPreset = updates.OIDCPreset
	}
	// AutoProvision is a boolean — always apply.
	existing.AutoProvision = updates.AutoProvision

	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteProvider performs a soft delete on an authentication provider
// within accountID.
func (s *AuthProviderService) DeleteProvider(ctx context.Context, id, accountID uint) error {
	if _, err := s.findScoped(ctx, id, accountID); err != nil {
		return err
	}

	return s.repo.Delete(ctx, id)
}

// ListProviders returns a paginated list of authentication providers for an account.
func (s *AuthProviderService) ListProviders(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuthenticationProvider], error) {
	return s.repo.ListByAccountID(ctx, accountID, params)
}

// TestLDAPConnection validates that the LDAP provider has the required configuration
// fields set. This is a stub that checks configuration completeness without actually
// connecting to the LDAP server.
func (s *AuthProviderService) TestLDAPConnection(ctx context.Context, id, accountID uint) (*LDAPTestResult, error) {
	provider, err := s.findScoped(ctx, id, accountID)
	if err != nil {
		return nil, err
	}

	if provider.AuthType != "ldap" {
		return &LDAPTestResult{
			Success: false,
			Message: "Provider is not an LDAP provider",
		}, nil
	}

	if provider.LDAPHost == "" {
		return &LDAPTestResult{
			Success: false,
			Message: "LDAP host is not configured",
		}, nil
	}

	if provider.LDAPPort == 0 {
		return &LDAPTestResult{
			Success: false,
			Message: "LDAP port is not configured",
		}, nil
	}

	if provider.LDAPBase == "" {
		return &LDAPTestResult{
			Success: false,
			Message: "LDAP base DN is not configured",
		}, nil
	}

	return &LDAPTestResult{
		Success: true,
		Message: "LDAP configuration is valid. Connection test will be available when LDAP protocol support is implemented.",
	}, nil
}

func isValidAuthType(authType string) bool {
	return authType == "saml" || authType == "ldap" || authType == "cas" || authType == "oidc"
}
