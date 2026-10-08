package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// clientSecretHashPrefix marks a developer_keys.client_secret value that
// holds a SHA-256 digest rather than the secret itself (B191). Migration
// 000095 rewrites legacy plaintext rows into this form.
const clientSecretHashPrefix = "sha256:"

// HashClientSecret returns the at-rest form of an OAuth2 client secret.
// The secret is 256 bits of CSPRNG output, so an unsalted fast hash is
// sufficient: there is nothing to brute-force.
func HashClientSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return clientSecretHashPrefix + hex.EncodeToString(sum[:])
}

// DeveloperKeyService manages OAuth2 developer keys (client applications).
type DeveloperKeyService struct {
	devKeyRepo repository.DeveloperKeyRepository
}

// NewDeveloperKeyService creates a new DeveloperKeyService.
func NewDeveloperKeyService(devKeyRepo repository.DeveloperKeyRepository) *DeveloperKeyService {
	return &DeveloperKeyService{devKeyRepo: devKeyRepo}
}

// Create generates a new developer key with a random ClientID and
// ClientSecret. It returns the raw client secret: only its hash is
// persisted, so the caller must hand the raw value to the admin now or
// never. The Name field is required.
func (s *DeveloperKeyService) Create(ctx context.Context, key *models.DeveloperKey) (string, error) {
	if key.Name == "" {
		return "", errors.New("developer key name is required")
	}

	// Generate ClientID: "dk_" + UUID
	key.ClientID = "dk_" + uuid.New().String()

	// Generate ClientSecret: random 64-character hex string (32 bytes)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", errors.New("failed to generate client secret")
	}
	rawSecret := hex.EncodeToString(secretBytes)
	key.ClientSecret = HashClientSecret(rawSecret)

	// Set default workflow state if not provided
	if key.WorkflowState == "" {
		key.WorkflowState = "active"
	}

	if err := s.devKeyRepo.Create(ctx, key); err != nil {
		return "", err
	}
	return rawSecret, nil
}

// VerifyClientSecret reports whether presented is the key's client secret.
// Comparison is constant-time. Legacy rows that still hold the plaintext
// (written before migration 000095) are compared directly.
func (s *DeveloperKeyService) VerifyClientSecret(key *models.DeveloperKey, presented string) bool {
	return VerifyClientSecret(key, presented)
}

// VerifyClientSecret is the package-level form used by services that hold
// a *models.DeveloperKey but not a DeveloperKeyService.
func VerifyClientSecret(key *models.DeveloperKey, presented string) bool {
	if key == nil || presented == "" || key.ClientSecret == "" {
		return false
	}
	stored := key.ClientSecret
	candidate := presented
	if strings.HasPrefix(stored, clientSecretHashPrefix) {
		candidate = HashClientSecret(presented)
	}
	return subtle.ConstantTimeCompare([]byte(stored), []byte(candidate)) == 1
}

// GetByID retrieves a developer key by its primary key ID, scoped to the
// caller's tenant. accountID==0 skips the filter (internal background callers
// such as the OAuth2 token exchange path or the LTI launch builder).
func (s *DeveloperKeyService) GetByID(ctx context.Context, id, accountID uint) (*models.DeveloperKey, error) {
	key, err := s.devKeyRepo.FindByID(ctx, id, accountID)
	if err != nil {
		return nil, errors.New("developer key not found")
	}
	return key, nil
}

// GetByClientID retrieves a developer key by its OAuth2 client_id.
func (s *DeveloperKeyService) GetByClientID(ctx context.Context, clientID string) (*models.DeveloperKey, error) {
	key, err := s.devKeyRepo.FindByClientID(ctx, clientID)
	if err != nil {
		return nil, errors.New("developer key not found")
	}
	return key, nil
}

// Update modifies a developer key. ClientID and ClientSecret cannot be changed.
// Tenant-scoped via accountID; accountID==0 skips the filter (internal callers).
func (s *DeveloperKeyService) Update(ctx context.Context, key *models.DeveloperKey, accountID uint) error {
	existing, err := s.devKeyRepo.FindByID(ctx, key.ID, accountID)
	if err != nil {
		return errors.New("developer key not found")
	}

	// Preserve the original ClientID and ClientSecret; they must never change
	key.ClientID = existing.ClientID
	key.ClientSecret = existing.ClientSecret
	// The owning tenant is immutable too.
	key.AccountID = existing.AccountID

	return s.devKeyRepo.Update(ctx, key)
}

// Delete performs a soft delete by setting workflow_state to "deleted".
// Tenant-scoped via accountID; accountID==0 skips the filter (internal callers).
func (s *DeveloperKeyService) Delete(ctx context.Context, id, accountID uint) error {
	if _, err := s.devKeyRepo.FindByID(ctx, id, accountID); err != nil {
		return errors.New("developer key not found")
	}

	return s.devKeyRepo.Delete(ctx, id, accountID)
}

// List returns a paginated list of developer keys for the given account.
func (s *DeveloperKeyService) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DeveloperKey], error) {
	return s.devKeyRepo.List(ctx, accountID, params)
}

// ValidateRedirectURI checks whether the given URI is an allowed redirect for
// this developer key. It matches against the key's RedirectURI (single value)
// and the newline-separated list in RedirectURIs.
func (s *DeveloperKeyService) ValidateRedirectURI(key *models.DeveloperKey, uri string) bool {
	return DeveloperKeyAllowsRedirect(key, uri)
}

// DeveloperKeyAllowsRedirect is the package-level redirect-URI allow-list
// check, shared with the LTI OIDC authorization endpoint.
func DeveloperKeyAllowsRedirect(key *models.DeveloperKey, uri string) bool {
	if uri == "" || key == nil {
		return false
	}

	// Check the primary redirect URI
	if key.RedirectURI != "" && key.RedirectURI == uri {
		return true
	}

	// Check the newline-separated list of additional redirect URIs
	if key.RedirectURIs != "" {
		uris := strings.Split(key.RedirectURIs, "\n")
		for _, allowed := range uris {
			if strings.TrimSpace(allowed) == uri {
				return true
			}
		}
	}

	return false
}
