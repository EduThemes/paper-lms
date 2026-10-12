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
	"github.com/EduThemes/paper-lms/internal/repository"
)

// authorizationCodeTTL is how long an authorization code stays exchangeable.
const authorizationCodeTTL = 10 * time.Minute

// OAuth2Service handles the OAuth2 authorization code flow.
type OAuth2Service struct {
	devKeyService *DeveloperKeyService
	tokenService  *AccessTokenService
	// codes stores authorization codes. Production wires the Postgres
	// repo via SetCodeRepository so any replica can redeem a code minted
	// on any other (B192); the in-memory default serves tests and
	// single-process dev, and purges expired codes on every write.
	codes repository.OAuthAuthorizationCodeRepository
}

// NewOAuth2Service creates a new OAuth2Service backed by an in-memory code
// store. Call SetCodeRepository to share codes across replicas.
func NewOAuth2Service(devKeyService *DeveloperKeyService, tokenService *AccessTokenService) *OAuth2Service {
	return &OAuth2Service{
		devKeyService: devKeyService,
		tokenService:  tokenService,
		codes:         newMemoryAuthCodeStore(),
	}
}

// SetCodeRepository swaps the authorization-code store (nil is ignored).
func (s *OAuth2Service) SetCodeRepository(repo repository.OAuthAuthorizationCodeRepository) {
	if repo != nil {
		s.codes = repo
	}
}

// ErrOAuthClientNotFound is returned when a client_id is unknown, inactive,
// or owned by a different tenant than the authorizing user. Handlers map it
// to a generic "invalid client" error so tenants can't be probed.
var ErrOAuthClientNotFound = errors.New("invalid client_id")

// ResolveClientForUser loads the developer key behind clientID and checks
// it may be authorized by a user of userAccountID: it must exist, be
// active, belong to the same tenant (B076: developer keys are tenant-owned;
// a key minted in tenant A must not harvest tokens from tenant B users),
// and list redirectURI.
func (s *OAuth2Service) ResolveClientForUser(ctx context.Context, clientID, redirectURI string, userAccountID uint) (*models.DeveloperKey, error) {
	devKey, err := s.devKeyService.GetByClientID(ctx, clientID)
	if err != nil || devKey.WorkflowState != "active" {
		return nil, ErrOAuthClientNotFound
	}
	if userAccountID == 0 || devKey.AccountID != userAccountID {
		return nil, ErrOAuthClientNotFound
	}
	if !s.devKeyService.ValidateRedirectURI(devKey, redirectURI) {
		return nil, errors.New("invalid redirect_uri")
	}
	return devKey, nil
}

// GenerateAuthorizationCode validates the developer key and redirect URI,
// then creates a short-lived authorization code that can later be exchanged
// for tokens. Callers MUST only invoke this after the user explicitly
// consented (POST /login/oauth2/auth) — never from a GET (B076).
func (s *OAuth2Service) GenerateAuthorizationCode(ctx context.Context, userID, userAccountID uint, clientID string, redirectURI string, scopes []string) (string, error) {
	devKey, err := s.ResolveClientForUser(ctx, clientID, redirectURI, userAccountID)
	if err != nil {
		return "", err
	}

	// Validate scopes if the developer key requires them. Canvas refuses a
	// scope-required key's authorization request that names no scopes.
	if devKey.RequireScopes {
		if len(scopes) == 0 {
			return "", errors.New("scope is required for this developer key")
		}
		if _, err := s.ValidateScopes(scopes, ParseScopes(devKey.Scopes)); err != nil {
			return "", err
		}
	}

	// Generate a random authorization code
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", errors.New("failed to generate authorization code")
	}
	code := hex.EncodeToString(codeBytes)

	// Opportunistic purge so abandoned codes never accumulate.
	_ = s.codes.DeleteExpired(ctx)

	row := &models.OAuthAuthorizationCode{
		CodeHash:       hashToken(code),
		UserID:         userID,
		DeveloperKeyID: devKey.ID,
		ClientID:       clientID,
		RedirectURI:    redirectURI,
		Scopes:         ScopesToJSON(scopes),
		ExpiresAt:      time.Now().Add(authorizationCodeTTL),
	}
	if err := s.codes.Create(ctx, row); err != nil {
		return "", errors.New("failed to store authorization code")
	}

	return code, nil
}

// ExchangeCode validates the authorization code and client credentials, then
// exchanges the code for an OAuth2 access token and refresh token. The code
// is consumed (single-use). Returns the token model, raw access token, and
// raw refresh token.
func (s *OAuth2Service) ExchangeCode(ctx context.Context, code string, clientID string, clientSecret string, redirectURI string) (*models.AccessToken, string, string, error) {
	// Look up and consume the authorization code
	ac, err := s.codes.Consume(ctx, hashToken(code))
	if err != nil || ac == nil {
		return nil, "", "", errors.New("invalid or expired authorization code")
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

	if !VerifyClientSecret(devKey, clientSecret) {
		return nil, "", "", errors.New("invalid client_secret")
	}

	if devKey.WorkflowState != "active" {
		return nil, "", "", errors.New("developer key is not active")
	}

	// Create the OAuth token
	token, rawAccess, rawRefresh, err := s.tokenService.CreateOAuthToken(ctx, ac.UserID, devKey.ID, ParseScopes(ac.Scopes))
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

// memoryAuthCodeStore is the single-process fallback code store.
type memoryAuthCodeStore struct {
	mu    sync.Mutex
	codes map[string]models.OAuthAuthorizationCode
}

func newMemoryAuthCodeStore() *memoryAuthCodeStore {
	return &memoryAuthCodeStore{codes: map[string]models.OAuthAuthorizationCode{}}
}

func (m *memoryAuthCodeStore) Create(_ context.Context, code *models.OAuthAuthorizationCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	code.CreatedAt = time.Now()
	m.codes[code.CodeHash] = *code
	return nil
}

func (m *memoryAuthCodeStore) Consume(_ context.Context, codeHash string) (*models.OAuthAuthorizationCode, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	row, ok := m.codes[codeHash]
	if !ok {
		return nil, errors.New("authorization code not found")
	}
	delete(m.codes, codeHash)
	return &row, nil
}

func (m *memoryAuthCodeStore) DeleteExpired(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, v := range m.codes {
		if now.After(v.ExpiresAt) {
			delete(m.codes, k)
		}
	}
	return nil
}
