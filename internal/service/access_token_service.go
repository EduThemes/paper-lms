package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// OAuthAccessTokenTTL is the lifetime of an OAuth2 access token. Refresh
// tokens do not expire on their own; they die with the token row.
const OAuthAccessTokenTTL = 1 * time.Hour

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

	token := &models.AccessToken{
		UserID:        userID,
		Token:         tokenHash,
		TokenHint:     hint,
		Scopes:        ScopesToJSON(scopes),
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

	expiresAt := time.Now().Add(OAuthAccessTokenTTL)

	token := &models.AccessToken{
		UserID:         userID,
		DeveloperKeyID: &devKeyID,
		Token:          accessHash,
		TokenHint:      hint,
		RefreshToken:   &refreshHash,
		Scopes:         ScopesToJSON(scopes),
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
//
// B183: an OAuth2 token is only as alive as its developer key. The repo
// preloads token.DeveloperKey; a key that is missing, deactivated or
// deleted kills every token minted for it immediately (Canvas parity: an
// inactive key's tokens are rejected).
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

	if token.DeveloperKeyID != nil {
		if token.DeveloperKey == nil || token.DeveloperKey.WorkflowState != "active" {
			return nil, errors.New("developer key is not active")
		}
	}

	// Update LastUsedAt. Prefer the single-column write: a full-row Save
	// of the copy read above would race a concurrent refresh (B193 updates
	// the access-token hash in place) or revoke, writing the stale hash /
	// workflow_state back over it. Non-fatal either way: the token is
	// still valid even if usage can't be recorded.
	now := time.Now()
	token.LastUsedAt = &now
	if rec, ok := s.tokenRepo.(accessTokenUsageRecorder); ok {
		_ = rec.TouchLastUsed(ctx, token.ID, now)
	} else {
		_ = s.tokenRepo.Update(ctx, token)
	}

	return token, nil
}

// accessTokenUsageRecorder is implemented by token repositories that can
// record a use without rewriting the rest of the row (the Postgres repo).
type accessTokenUsageRecorder interface {
	TouchLastUsed(ctx context.Context, id uint, at time.Time) error
}

// RefreshOAuthToken uses a raw refresh token to mint a new access token.
//
// B184: the refresh token must have been issued to the authenticating
// client (devKeyID); presenting another client's refresh token fails.
//
// B193 (Canvas parity): the refresh token is NOT rotated. Canvas keeps the
// original refresh token valid for the life of the grant and its refresh
// response carries no new refresh_token, so Canvas-API clients store the
// first one forever. The token row is updated in place with a new access
// token hash and expiry; the previous access token stops working.
func (s *AccessTokenService) RefreshOAuthToken(ctx context.Context, rawRefreshToken string, devKeyID uint) (*models.AccessToken, string, error) {
	refreshHash := hashToken(rawRefreshToken)

	token, err := s.tokenRepo.FindByRefreshToken(ctx, refreshHash)
	if err != nil {
		return nil, "", errors.New("invalid refresh token")
	}

	if token.WorkflowState != "active" {
		return nil, "", errors.New("refresh token has been revoked")
	}

	// The refresh token must belong to an OAuth token (has a developer key)
	if token.DeveloperKeyID == nil {
		return nil, "", errors.New("refresh token is not associated with an OAuth token")
	}
	if *token.DeveloperKeyID != devKeyID {
		// Same message as an unknown token: don't confirm to client B
		// that client A's refresh token exists.
		return nil, "", errors.New("invalid refresh token")
	}

	rawAccessToken, err := generateRandomHex(32)
	if err != nil {
		return nil, "", errors.New("failed to generate access token")
	}
	expiresAt := time.Now().Add(OAuthAccessTokenTTL)
	token.Token = hashToken(rawAccessToken)
	token.TokenHint = rawAccessToken[len(rawAccessToken)-4:]
	token.ExpiresAt = &expiresAt

	if err := s.tokenRepo.Update(ctx, token); err != nil {
		return nil, "", errors.New("failed to refresh token")
	}

	return token, rawAccessToken, nil
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

// ScopesToJSON converts a slice of scope strings into the JSON-array string
// stored in access_tokens.scopes / developer_keys.scopes.
func ScopesToJSON(scopes []string) string {
	if len(scopes) == 0 {
		return "[]"
	}
	b, err := json.Marshal(scopes)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ParseScopes parses a stored JSON array string like `["a","b"]` into a
// string slice. Malformed legacy values fall back to a tolerant split.
func ParseScopes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "[]" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err == nil {
		return out
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	out = []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.Trim(strings.TrimSpace(p), "\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// TokenScopeRestriction reports whether the token is scope-restricted and,
// if so, the scopes it holds (B085).
//
//   - OAuth2 tokens follow Canvas: scopes are enforced only when the
//     developer key has require_scopes on.
//   - Personal access tokens are enforced whenever the user asked for
//     scopes at creation; a PAT created without scopes is full-access.
func TokenScopeRestriction(token *models.AccessToken) ([]string, bool) {
	if token == nil {
		return nil, false
	}
	scopes := ParseScopes(token.Scopes)
	if token.DeveloperKeyID != nil {
		if token.DeveloperKey != nil && token.DeveloperKey.RequireScopes {
			return scopes, true
		}
		return nil, false
	}
	if len(scopes) > 0 {
		return scopes, true
	}
	return nil, false
}

var validScopeMethods = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true, "HEAD": true}

// IsValidScope reports whether scope is a Canvas-format URL scope
// ("url:<METHOD>|/api/...").
func IsValidScope(scope string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(scope), "url:")
	if !ok {
		return false
	}
	method, path, ok := strings.Cut(rest, "|")
	return ok && validScopeMethods[strings.ToUpper(method)] && strings.HasPrefix(path, "/api/")
}

// scopeParamSegment matches a Canvas route-style placeholder segment.
var scopeParamSegment = regexp.MustCompile(`^:[A-Za-z_][A-Za-z0-9_]*$`)

// ScopeAllowsRequest reports whether any Canvas-format scope
// ("url:GET|/api/v1/courses/:course_id/assignments") permits the request.
// HEAD is treated as GET. ":param" segments match exactly one path
// segment, "*glob" matches the remainder, and Canvas's optional
// "(.:format)" suffix is ignored.
func ScopeAllowsRequest(scopes []string, method, path string) bool {
	method = strings.ToUpper(method)
	if method == "HEAD" {
		method = "GET"
	}
	path = strings.TrimSuffix(path, "/")
	for _, scope := range scopes {
		rest, ok := strings.CutPrefix(scope, "url:")
		if !ok {
			continue
		}
		scopeMethod, pattern, ok := strings.Cut(rest, "|")
		if !ok || strings.ToUpper(scopeMethod) != method {
			continue
		}
		pattern = strings.TrimSuffix(pattern, "(.:format)")
		pattern = strings.TrimSuffix(pattern, "/")
		if scopePathMatches(pattern, path) {
			return true
		}
	}
	return false
}

func scopePathMatches(pattern, path string) bool {
	ps := strings.Split(pattern, "/")
	rs := strings.Split(path, "/")
	for i, seg := range ps {
		if strings.HasPrefix(seg, "*") {
			return i < len(rs)
		}
		if i >= len(rs) {
			return false
		}
		if scopeParamSegment.MatchString(seg) {
			if rs[i] == "" {
				return false
			}
			continue
		}
		if seg != rs[i] {
			return false
		}
	}
	return len(ps) == len(rs)
}
