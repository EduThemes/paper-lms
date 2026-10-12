package postgres

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type loginSessionTokenRepo struct {
	db *gorm.DB
}

// NewLoginSessionTokenRepository stores /login/session_token credentials
// (G136).
func NewLoginSessionTokenRepository(db *gorm.DB) repository.LoginSessionTokenRepository {
	return &loginSessionTokenRepo{db: db}
}

func (r *loginSessionTokenRepo) Create(ctx context.Context, t *models.LoginSessionToken) error {
	return r.db.WithContext(ctx).Create(t).Error
}

// Consume deletes and returns the row in one statement, so a session URL
// can be used once only.
func (r *loginSessionTokenRepo) Consume(ctx context.Context, tokenHash string) (*models.LoginSessionToken, error) {
	var rows []models.LoginSessionToken
	res := r.db.WithContext(ctx).
		Clauses(clause.Returning{}).
		Where("token_hash = ?", tokenHash).
		Delete(&rows)
	if res.Error != nil {
		return nil, res.Error
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (r *loginSessionTokenRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.LoginSessionToken{}).Error
}
