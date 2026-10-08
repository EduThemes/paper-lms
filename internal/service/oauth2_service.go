package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
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
	return s.GenerateAuthorizationCodePKCE(ctx, userID, userAccountID, clientID, redirectURI, scopes, PKCEChallenge{})
}

// PKCEChallenge is the RFC 7636 code challenge a client sent with its
// authorization request. The zero value means no PKCE.
type PKCEChallenge struct {
	Challenge string
	Method    string
}

// PKCEMethodS256 is the only PKCE method accepted (as in Canvas): "plain"
// offers no protection against an intercepted code.
const PKCEMethodS256 = "S256"

// pkceChallengePattern is RFC 7636's code_challenge: the base64url
// (unpadded) SHA-256 of the verifier, 43 characters.
var pkceChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// pkceVerifierPattern is RFC 7636 section 4.1's code_verifier alphabet and length.
var pkceVerifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

// ErrPKCEInvalid is returned for a malformed or unsupported PKCE request.
var ErrPKCEInvalid = errors.New("invalid PKCE code_challenge: send a 43-character base64url S256 challenge with code_challenge_method=S256")

// Validate checks a PKCE challenge. The zero value is valid (no PKCE).
func (p PKCEChallenge) Validate() error {
	if p.Challenge == "" && p.Method == "" {
		return nil
	}
	if p.Method != PKCEMethodS256 || !pkceChallengePattern.MatchString(p.Challenge) {
		return ErrPKCEInvalid
	}
	return nil
}

// PKCEVerifies reports whether verifier hashes to challenge (S256).
func PKCEVerifies(challenge, verifier string) bool {
	if !pkceVerifierPattern.MatchString(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(challenge)) == 1
}

// GenerateAuthorizationCodePKCE is GenerateAuthorizationCode with an
// optional PKCE challenge stored alongside the code (G136).
func (s *OAuth2Service) GenerateAuthorizationCodePKCE(ctx context.Context, userID, userAccountID uint, clientID string, redirectURI string, scopes []string, pkce PKCEChallenge) (string, error) {
	if err := pkce.Validate(); err != nil {
		return "", err
	}
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
	if pkce.Challenge != "" {
		challenge, method := pkce.Challenge, pkce.Method
		row.CodeChallenge = &challenge
		row.CodeChallengeMethod = &method
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
	return s.ExchangeCodePKCE(ctx, code, clientID, clientSecret, redirectURI, "")
}

// ErrOAuthInvalidClient is returned when the client cannot be
// authenticated (unknown client, wrong secret, or a missing secret
// without PKCE). Handlers map it to OAuth's invalid_client.
var ErrOAuthInvalidClient = errors.New("invalid client credentials")

// ExchangeCodePKCE exchanges an authorization code, enforcing PKCE (G136):
//
//   - a code issued with a code_challenge can only be redeemed with the
//     matching code_verifier (RFC 7636 section 4.6), and
//   - such a client may omit client_secret (a public client, e.g. a
//     native or single-page app). Without PKCE the secret is required.
func (s *OAuth2Service) ExchangeCodePKCE(ctx context.Context, code string, clientID string, clientSecret string, redirectURI string, codeVerifier string) (*models.AccessToken, string, string, error) {
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

	usedPKCE := false
	if ac.CodeChallenge != nil && *ac.CodeChallenge != "" {
		if codeVerifier == "" || !PKCEVerifies(*ac.CodeChallenge, codeVerifier) {
			return nil, "", "", errors.New("invalid code_verifier")
		}
		usedPKCE = true
	} else if codeVerifier != "" {
		// A verifier for a code issued without a challenge proves nothing.
		return nil, "", "", errors.New("code_verifier sent for a code issued without code_challenge")
	}

	if clientSecret != "" || !usedPKCE {
		if !VerifyClientSecret(devKey, clientSecret) {
			return nil, "", "", ErrOAuthInvalidClient
		}
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
