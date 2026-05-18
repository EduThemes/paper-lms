package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type developerKeyRepo struct {
	db *gorm.DB
}

func NewDeveloperKeyRepository(db *gorm.DB) repository.DeveloperKeyRepository {
	return &developerKeyRepo{db: db}
}

func (r *developerKeyRepo) Create(ctx context.Context, key *models.DeveloperKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *developerKeyRepo) FindByID(ctx context.Context, id, accountID uint) (*models.DeveloperKey, error) {
	var key models.DeveloperKey
	q := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted")
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	if err := q.First(&key, id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *developerKeyRepo) FindByClientID(ctx context.Context, clientID string) (*models.DeveloperKey, error) {
	var key models.DeveloperKey
	if err := r.db.WithContext(ctx).Where("client_id = ? AND workflow_state != ?", clientID, "deleted").First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

func (r *developerKeyRepo) Update(ctx context.Context, key *models.DeveloperKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

func (r *developerKeyRepo) Delete(ctx context.Context, id, accountID uint) error {
	q := r.db.WithContext(ctx).Model(&models.DeveloperKey{}).Where("id = ?", id)
	if accountID != 0 {
		q = q.Where("account_id = ?", accountID)
	}
	return q.Update("workflow_state", "deleted").Error
}

func (r *developerKeyRepo) List(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.DeveloperKey], error) {
	var keys []models.DeveloperKey
	var count int64

	query := r.db.WithContext(ctx).Model(&models.DeveloperKey{}).Where("account_id = ? AND workflow_state != ?", accountID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&keys).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.DeveloperKey]{
		Items:      keys,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
