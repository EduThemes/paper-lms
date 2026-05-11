package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type blueprintMigrationRepo struct {
	db *gorm.DB
}

func NewBlueprintMigrationRepository(db *gorm.DB) repository.BlueprintMigrationRepository {
	return &blueprintMigrationRepo{db: db}
}

func (r *blueprintMigrationRepo) Create(ctx context.Context, migration *models.BlueprintMigration) error {
	return r.db.WithContext(ctx).Create(migration).Error
}

func (r *blueprintMigrationRepo) FindByID(ctx context.Context, id uint) (*models.BlueprintMigration, error) {
	var migration models.BlueprintMigration
	if err := r.db.WithContext(ctx).First(&migration, id).Error; err != nil {
		return nil, err
	}
	return &migration, nil
}

func (r *blueprintMigrationRepo) Update(ctx context.Context, migration *models.BlueprintMigration) error {
	return r.db.WithContext(ctx).Save(migration).Error
}

func (r *blueprintMigrationRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.BlueprintMigration{}, id).Error
}

func (r *blueprintMigrationRepo) ListByTemplateID(ctx context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintMigration], error) {
	var migrations []models.BlueprintMigration
	var count int64

	query := r.db.WithContext(ctx).Model(&models.BlueprintMigration{}).Where("blueprint_template_id = ?", templateID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&migrations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.BlueprintMigration]{
		Items:      migrations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *blueprintMigrationRepo) ListBySubscriptionID(ctx context.Context, subscriptionID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintMigration], error) {
	var migrations []models.BlueprintMigration
	var count int64

	// Get the subscription to find its template
	var subscription models.BlueprintSubscription
	if err := r.db.WithContext(ctx).First(&subscription, subscriptionID).Error; err != nil {
		return nil, err
	}

	query := r.db.WithContext(ctx).Model(&models.BlueprintMigration{}).Where("blueprint_template_id = ?", subscription.BlueprintTemplateID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&migrations).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.BlueprintMigration]{
		Items:      migrations,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
