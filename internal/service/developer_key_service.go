package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// DeveloperKeyService manages OAuth2 developer keys (client applications).
type DeveloperKeyService struct {
	devKeyRepo repository.DeveloperKeyRepository
}

// NewDeveloperKeyService creates a new DeveloperKeyService.
func NewDeveloperKeyService(devKeyRepo repository.DeveloperKeyRepository) *DeveloperKeyService {
	return &DeveloperKeyService{devKeyRepo: devKeyRepo}
}

// Create generates a new developer key with a random ClientID and ClientSecret.
// The Name field is required; if missing, an error is returned.
func (s *DeveloperKeyService) Create(ctx context.Context, key *models.DeveloperKey) error {
	if key.Name == "" {
		return errors.New("developer key name is required")
	}

	// Generate ClientID: "dk_" + UUID
	key.ClientID = "dk_" + uuid.New().String()

	// Generate ClientSecret: random 64-character hex string (32 bytes)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return errors.New("failed to generate client secret")
	}
	key.ClientSecret = hex.EncodeToString(secretBytes)

	// Set default workflow state if not provided
	if key.WorkflowState == "" {
		key.WorkflowState = "active"
	}

	return s.devKeyRepo.Create(ctx, key)
}

// GetByID retrieves a developer key by its primary key ID.
func (s *DeveloperKeyService) GetByID(ctx context.Context, id uint) (*models.DeveloperKey, error) {
	key, err := s.devKeyRepo.FindByID(ctx, id)
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
func (s *DeveloperKeyService) Update(ctx context.Context, key *models.DeveloperKey) error {
	existing, err := s.devKeyRepo.FindByID(ctx, key.ID)
	if err != nil {
		return errors.New("developer key not found")
	}

	// Preserve the original ClientID and ClientSecret; they must never change
	key.ClientID = existing.ClientID
	key.ClientSecret = existing.ClientSecret

	return s.devKeyRepo.Update(ctx, key)
}

// Delete performs a soft delete by setting workflow_state to "deleted".
func (s *DeveloperKeyService) Delete(ctx context.Context, id uint) error {
	key, err := s.devKeyRepo.FindByID(ctx, id)
	if err != nil {
		return errors.New("developer key not found")
	}

	key.WorkflowState = "deleted"
	return s.devKeyRepo.Update(ctx, key)
}

// List returns a paginated list of developer keys for the given account.
func (s *DeveloperKeyService) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DeveloperKey], error) {
	return s.devKeyRepo.List(ctx, accountID, params)
}

// ValidateRedirectURI checks whether the given URI is an allowed redirect for
// this developer key. It matches against the key's RedirectURI (single value)
// and the newline-separated list in RedirectURIs.
func (s *DeveloperKeyService) ValidateRedirectURI(key *models.DeveloperKey, uri string) bool {
	if uri == "" {
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
