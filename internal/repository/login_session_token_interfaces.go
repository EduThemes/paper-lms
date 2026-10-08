package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// LoginSessionTokenRepository stores the single-use credentials behind
// GET /login/session_token (G136).
type LoginSessionTokenRepository interface {
	Create(ctx context.Context, t *models.LoginSessionToken) error
	// Consume atomically deletes and returns the row; a miss is an error.
	Consume(ctx context.Context, tokenHash string) (*models.LoginSessionToken, error)
	DeleteExpired(ctx context.Context) error
}
