package postgres

import (
	"context"
	"time"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type accessTokenRepo struct {
	db *gorm.DB
}

func NewAccessTokenRepository(db *gorm.DB) repository.AccessTokenRepository {
	return &accessTokenRepo{db: db}
}

func (r *accessTokenRepo) Create(ctx context.Context, token *models.AccessToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *accessTokenRepo) FindByID(ctx context.Context, id uint) (*models.AccessToken, error) {
	var token models.AccessToken
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&token, id).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *accessTokenRepo) FindByToken(ctx context.Context, tokenHash string) (*models.AccessToken, error) {
	var token models.AccessToken
	if err := r.db.WithContext(ctx).Where("token = ? AND workflow_state != ?", tokenHash, "deleted").First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *accessTokenRepo) FindByRefreshToken(ctx context.Context, refreshToken string) (*models.AccessToken, error) {
	var token models.AccessToken
	if err := r.db.WithContext(ctx).Where("refresh_token = ? AND workflow_state != ?", refreshToken, "deleted").First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

func (r *accessTokenRepo) Update(ctx context.Context, token *models.AccessToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

func (r *accessTokenRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.AccessToken{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *accessTokenRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AccessToken], error) {
	var tokens []models.AccessToken
	var count int64

	query := r.db.WithContext(ctx).Model(&models.AccessToken{}).Where("user_id = ? AND workflow_state != ?", userID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&tokens).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AccessToken]{
		Items:      tokens,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *accessTokenRepo) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at IS NOT NULL AND expires_at < ?", time.Now()).Delete(&models.AccessToken{}).Error
}
