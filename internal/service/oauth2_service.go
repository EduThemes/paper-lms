package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// authorizationCode holds the data associated with a short-lived OAuth2
// authorization code. Codes expire after 10 minutes.
type authorizationCode struct {
	Code        string
	UserID      uint
	ClientID    string
	RedirectURI string
	Scopes      []string
	ExpiresAt   time.Time
}

// OAuth2Service handles the OAuth2 authorization code flow.
type OAuth2Service struct {
	devKeyService *DeveloperKeyService
	tokenService  *AccessTokenService
	// codes stores authorization codes keyed by the code string.
	// A sync.Map is used for concurrent access safety.
	codes sync.Map
}

// NewOAuth2Service creates a new OAuth2Service.
func NewOAuth2Service(devKeyService *DeveloperKeyService, tokenService *AccessTokenService) *OAuth2Service {
	return &OAuth2Service{
		devKeyService: devKeyService,
		tokenService:  tokenService,
	}
}

// GenerateAuthorizationCode validates the developer key and redirect URI, then
// creates a short-lived authorization code that can later be exchanged for
// tokens. The code expires after 10 minutes.
func (s *OAuth2Service) GenerateAuthorizationCode(ctx context.Context, userID uint, clientID string, redirectURI string, scopes []string, state string) (string, error) {
	// Validate that the developer key exists and is active
	devKey, err := s.devKeyService.GetByClientID(ctx, clientID)
	if err != nil {
		return "", errors.New("invalid client_id")
	}

	if devKey.WorkflowState != "active" {
		return "", errors.New("developer key is not active")
	}

	// Validate the redirect URI
	if !s.devKeyService.ValidateRedirectURI(devKey, redirectURI) {
		return "", errors.New("invalid redirect_uri")
	}

	// Validate scopes if the developer key requires them
	if devKey.RequireScopes {
		allowedScopes := parseScopesJSON(devKey.Scopes)
		if _, err := s.ValidateScopes(scopes, allowedScopes); err != nil {
			return "", err
		}
	}

	// Generate a random authorization code
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", errors.New("failed to generate authorization code")
	}
	code := hex.EncodeToString(codeBytes)

	// Store the code with a 10-minute expiration
	ac := &authorizationCode{
		Code:        code,
		UserID:      userID,
		ClientID:    clientID,
		RedirectURI: redirectURI,
		Scopes:      scopes,
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	s.codes.Store(code, ac)

	return code, nil
}

// ExchangeCode validates the authorization code and client credentials, then
// exchanges the code for an OAuth2 access token and refresh token. The code
// is consumed (single-use). Returns the token model, raw access token, and
// raw refresh token.
func (s *OAuth2Service) ExchangeCode(ctx context.Context, code string, clientID string, clientSecret string, redirectURI string) (*models.AccessToken, string, string, error) {
	// Look up and consume the authorization code
	val, ok := s.codes.LoadAndDelete(code)
	if !ok {
		return nil, "", "", errors.New("invalid or expired authorization code")
	}

	ac, ok := val.(*authorizationCode)
	if !ok {
		return nil, "", "", errors.New("internal error: invalid code data")
	}

	// Check expiration
	if time.Now().After(ac.ExpiresAt) {
		return nil, "", "", errors.New("authorization code has expired")
	}

	// Verify that the client_id matches
	if ac.ClientID != clientID {
		return nil, "", "", errors.New("client_id mismatch")
	}

	// Verify that the redirect_uri matches
	if ac.RedirectURI != redirectURI {
		return nil, "", "", errors.New("redirect_uri mismatch")
	}

	// Verify client credentials
	devKey, err := s.devKeyService.GetByClientID(ctx, clientID)
	if err != nil {
		return nil, "", "", errors.New("invalid client_id")
	}

	if devKey.ClientSecret != clientSecret {
		return nil, "", "", errors.New("invalid client_secret")
	}

	if devKey.WorkflowState != "active" {
		return nil, "", "", errors.New("developer key is not active")
	}

	// Create the OAuth token
	token, rawAccess, rawRefresh, err := s.tokenService.CreateOAuthToken(ctx, ac.UserID, devKey.ID, ac.Scopes)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to create token: %w", err)
	}

	return token, rawAccess, rawRefresh, nil
}

// ValidateScopes checks that every requested scope is present in the allowed
// set. It returns the intersection of requested and allowed scopes. If any
// requested scope is not allowed, an error is returned.
func (s *OAuth2Service) ValidateScopes(requested []string, allowed []string) ([]string, error) {
	if len(requested) == 0 {
		return nil, nil
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, scope := range allowed {
		allowedSet[scope] = true
	}

	var validated []string
	for _, scope := range requested {
		if !allowedSet[scope] {
			return nil, fmt.Errorf("scope %q is not allowed", scope)
		}
		validated = append(validated, scope)
	}

	return validated, nil
}
