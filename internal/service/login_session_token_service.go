package service

import (
	"context"
	"errors"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

// LoginSessionTokenTTL is how long a /login/session_token URL stays usable.
// The URL is meant to be opened straight away by the app that asked for it.
const LoginSessionTokenTTL = 60 * time.Second

// ErrLoginSessionTokenInvalid is returned for an unknown, used or expired
// session token.
var ErrLoginSessionTokenInvalid = errors.New("invalid or expired session token")

// LoginSessionTokenService mints and redeems the single-use credentials
// behind Canvas's GET /login/session_token (G136): an app holding an API
// access token turns it into a URL that signs the same user into a
// browser session. Redemption goes through auth.LoginPipeline in the
// handler; this service only stores and checks the token.
type LoginSessionTokenService struct {
	repo repository.LoginSessionTokenRepository
	now  func() time.Time
}

// NewLoginSessionTokenService wires the service.
func NewLoginSessionTokenService(repo repository.LoginSessionTokenRepository) *LoginSessionTokenService {
	return &LoginSessionTokenService{repo: repo, now: time.Now}
}

// Mint stores a new token for the user and returns the raw value (only
// its SHA-256 is kept). returnTo must already be a validated same-site
// path.
func (s *LoginSessionTokenService) Mint(ctx context.Context, userID, accountID uint, returnTo string) (string, error) {
	raw, err := generateRandomHex(32)
	if err != nil {
		return "", errors.New("failed to generate session token")
	}
	_ = s.repo.DeleteExpired(ctx)
	row := &models.LoginSessionToken{
		TokenHash: hashToken(raw),
		UserID:    userID,
		AccountID: accountID,
		ReturnTo:  returnTo,
		ExpiresAt: s.now().Add(LoginSessionTokenTTL),
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return "", err
	}
	return raw, nil
}

// Consume redeems a raw token once. A second use, or a use after the TTL,
// fails.
func (s *LoginSessionTokenService) Consume(ctx context.Context, raw string) (*models.LoginSessionToken, error) {
	if raw == "" {
		return nil, ErrLoginSessionTokenInvalid
	}
	row, err := s.repo.Consume(ctx, hashToken(raw))
	if err != nil || row == nil {
		return nil, ErrLoginSessionTokenInvalid
	}
	if s.now().After(row.ExpiresAt) {
		return nil, ErrLoginSessionTokenInvalid
	}
	return row, nil
}
