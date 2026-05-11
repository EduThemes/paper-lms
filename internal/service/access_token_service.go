package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// AccessTokenService manages OAuth2 tokens and Personal Access Tokens.
type AccessTokenService struct {
	tokenRepo repository.AccessTokenRepository
}

// NewAccessTokenService creates a new AccessTokenService.
func NewAccessTokenService(tokenRepo repository.AccessTokenRepository) *AccessTokenService {
	return &AccessTokenService{tokenRepo: tokenRepo}
}

// generateRandomHex produces a cryptographically random hex string of the
// specified byte length (the resulting string is twice as long).
func generateRandomHex(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashToken returns the SHA-256 hex digest of a raw token string.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// CreatePersonalAccessToken generates a new Personal Access Token for the given
// user. It returns the persisted model and the raw token string. The raw token
// is only available at creation time; afterwards only the hash is stored.
func (s *AccessTokenService) CreatePersonalAccessToken(ctx context.Context, userID uint, purpose string, scopes []string) (*models.AccessToken, string, error) {
	// Generate a random 64-character hex token (32 bytes)
	rawToken, err := generateRandomHex(32)
	if err != nil {
		return nil, "", errors.New("failed to generate token")
	}

	tokenHash := hashToken(rawToken)
	hint := rawToken[len(rawToken)-4:] // last 4 characters

	scopeStr := "[]"
	if len(scopes) > 0 {
		scopeStr = scopesToJSON(scopes)
	}

	token := &models.AccessToken{
		UserID:        userID,
		Token:         tokenHash,
		TokenHint:     hint,
		Scopes:        scopeStr,
		Purpose:       purpose,
		WorkflowState: "active",
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, "", err
	}

	return token, rawToken, nil
}

// CreateOAuthToken generates a new OAuth2 access token and refresh token for
// the given user/developer-key pair. Returns the model, raw access token, and
// raw refresh token.
func (s *AccessTokenService) CreateOAuthToken(ctx context.Context, userID uint, devKeyID uint, scopes []string) (*models.AccessToken, string, string, error) {
	// Generate access token
	rawAccessToken, err := generateRandomHex(32)
	if err != nil {
		return nil, "", "", errors.New("failed to generate access token")
	}

	// Generate refresh token
	rawRefreshToken, err := generateRandomHex(32)
	if err != nil {
		return nil, "", "", errors.New("failed to generate refresh token")
	}

	accessHash := hashToken(rawAccessToken)
	refreshHash := hashToken(rawRefreshToken)
	hint := rawAccessToken[len(rawAccessToken)-4:]

	scopeStr := "[]"
	if len(scopes) > 0 {
		scopeStr = scopesToJSON(scopes)
	}

	// OAuth tokens expire after 1 hour
	expiresAt := time.Now().Add(1 * time.Hour)

	token := &models.AccessToken{
		UserID:         userID,
		DeveloperKeyID: &devKeyID,
		Token:          accessHash,
		TokenHint:      hint,
		RefreshToken:   &refreshHash,
		Scopes:         scopeStr,
		ExpiresAt:      &expiresAt,
		WorkflowState:  "active",
	}

	if err := s.tokenRepo.Create(ctx, token); err != nil {
		return nil, "", "", err
	}

	return token, rawAccessToken, rawRefreshToken, nil
}

// ValidateToken hashes the provided raw token, looks it up in the database,
// and verifies that it has not expired or been deleted. On success it updates
// LastUsedAt and returns the token record.
func (s *AccessTokenService) ValidateToken(ctx context.Context, rawToken string) (*models.AccessToken, error) {
	tokenHash := hashToken(rawToken)

	token, err := s.tokenRepo.FindByToken(ctx, tokenHash)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	// Check that the token is active
	if token.WorkflowState != "active" {
		return nil, errors.New("token has been revoked")
	}

	// Check expiration
	if token.ExpiresAt != nil && time.Now().After(*token.ExpiresAt) {
		return nil, errors.New("token has expired")
	}

	// Update LastUsedAt
	now := time.Now()
	token.LastUsedAt = &now
	if err := s.tokenRepo.Update(ctx, token); err != nil {
		// Non-fatal: token is still valid even if we can't record usage
		_ = err
	}

	return token, nil
}

// RefreshOAuthToken uses a raw refresh token to rotate the access and refresh
// tokens. The old token record is deleted and a new one is created. Returns
// the new model, new raw access token, and new raw refresh token.
func (s *AccessTokenService) RefreshOAuthToken(ctx context.Context, rawRefreshToken string) (*models.AccessToken, string, string, error) {
	refreshHash := hashToken(rawRefreshToken)

	oldToken, err := s.tokenRepo.FindByRefreshToken(ctx, refreshHash)
	if err != nil {
		return nil, "", "", errors.New("invalid refresh token")
	}

	if oldToken.WorkflowState != "active" {
		return nil, "", "", errors.New("refresh token has been revoked")
	}

	// The refresh token must belong to an OAuth token (has a developer key)
	if oldToken.DeveloperKeyID == nil {
		return nil, "", "", errors.New("refresh token is not associated with an OAuth token")
	}

	devKeyID := *oldToken.DeveloperKeyID
	userID := oldToken.UserID

	// Parse existing scopes from the old token
	scopes := parseScopesJSON(oldToken.Scopes)

	// Delete the old token
	if err := s.tokenRepo.Delete(ctx, oldToken.ID); err != nil {
		return nil, "", "", errors.New("failed to revoke old token")
	}

	// Create a new OAuth token
	return s.CreateOAuthToken(ctx, userID, devKeyID, scopes)
}

// RevokeToken marks a token as deleted. It verifies that the requesting user
// owns the token before revoking it.
func (s *AccessTokenService) RevokeToken(ctx context.Context, tokenID uint, userID uint) error {
	token, err := s.tokenRepo.FindByID(ctx, tokenID)
	if err != nil {
		return errors.New("token not found")
	}

	if token.UserID != userID {
		return errors.New("not authorized to revoke this token")
	}

	token.WorkflowState = "deleted"
	return s.tokenRepo.Update(ctx, token)
}

// ListUserTokens returns a paginated list of tokens belonging to the given user.
func (s *AccessTokenService) ListUserTokens(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AccessToken], error) {
	return s.tokenRepo.ListByUserID(ctx, userID, params)
}

// scopesToJSON converts a slice of scope strings into a JSON array string.
func scopesToJSON(scopes []string) string {
	if len(scopes) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, scope := range scopes {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(scope)
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

// parseScopesJSON parses a JSON array string like `["a","b"]` into a string
// slice. This is intentionally simple and does not handle edge cases beyond
// the format produced by scopesToJSON.
func parseScopesJSON(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return nil
	}
	// Strip brackets
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "\"")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
