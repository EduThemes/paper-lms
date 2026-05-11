package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type authProviderRepo struct {
	db *gorm.DB
}

func NewAuthenticationProviderRepository(db *gorm.DB) repository.AuthenticationProviderRepository {
	return &authProviderRepo{db: db}
}

func (r *authProviderRepo) Create(ctx context.Context, provider *models.AuthenticationProvider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

func (r *authProviderRepo) FindByID(ctx context.Context, id uint) (*models.AuthenticationProvider, error) {
	var provider models.AuthenticationProvider
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&provider, id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (r *authProviderRepo) Update(ctx context.Context, provider *models.AuthenticationProvider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

func (r *authProviderRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.AuthenticationProvider{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *authProviderRepo) ListByAccountID(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.AuthenticationProvider], error) {
	var providers []models.AuthenticationProvider
	var count int64

	query := r.db.WithContext(ctx).Model(&models.AuthenticationProvider{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("position ASC, id ASC").Find(&providers).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.AuthenticationProvider]{
		Items:      providers,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *authProviderRepo) FindByAccountAndType(ctx context.Context, accountID uint, authType string) ([]models.AuthenticationProvider, error) {
	var providers []models.AuthenticationProvider
	if err := r.db.WithContext(ctx).Where("account_id = ? AND auth_type = ? AND workflow_state != ?", accountID, authType, "deleted").Order("position ASC, id ASC").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}
