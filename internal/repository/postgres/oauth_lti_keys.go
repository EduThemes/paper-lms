package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type oauthAuthorizationCodeRepo struct {
	db *gorm.DB
}

// NewOAuthAuthorizationCodeRepository returns the Postgres-backed OAuth2
// authorization-code store (B192).
func NewOAuthAuthorizationCodeRepository(db *gorm.DB) repository.OAuthAuthorizationCodeRepository {
	return &oauthAuthorizationCodeRepo{db: db}
}

func (r *oauthAuthorizationCodeRepo) Create(ctx context.Context, code *models.OAuthAuthorizationCode) error {
	return r.db.WithContext(ctx).Create(code).Error
}

// Consume deletes the row and returns it in one statement, so two
// concurrent exchanges of the same code cannot both succeed.
func (r *oauthAuthorizationCodeRepo) Consume(ctx context.Context, codeHash string) (*models.OAuthAuthorizationCode, error) {
	var rows []models.OAuthAuthorizationCode
	res := r.db.WithContext(ctx).
		Clauses(clause.Returning{}).
		Where("code_hash = ?", codeHash).
		Delete(&rows)
	if res.Error != nil {
		return nil, res.Error
	}
	if len(rows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &rows[0], nil
}

func (r *oauthAuthorizationCodeRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&models.OAuthAuthorizationCode{}).Error
}

type ltiPlatformKeyRepo struct {
	db *gorm.DB
}

// NewLTIPlatformKeyRepository returns the Postgres-backed LTI platform
// signing-key store (B181).
func NewLTIPlatformKeyRepository(db *gorm.DB) repository.LTIPlatformKeyRepository {
	return &ltiPlatformKeyRepo{db: db}
}

func (r *ltiPlatformKeyRepo) FindByName(ctx context.Context, name string) (*models.LTIPlatformKey, error) {
	var key models.LTIPlatformKey
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *ltiPlatformKeyRepo) CreateIfAbsent(ctx context.Context, key *models.LTIPlatformKey) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "name"}}, DoNothing: true}).
		Create(key).Error
}
