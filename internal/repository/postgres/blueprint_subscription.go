package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type blueprintSubscriptionRepo struct {
	db *gorm.DB
}

func NewBlueprintSubscriptionRepository(db *gorm.DB) repository.BlueprintSubscriptionRepository {
	return &blueprintSubscriptionRepo{db: db}
}

func (r *blueprintSubscriptionRepo) Create(ctx context.Context, subscription *models.BlueprintSubscription) error {
	return r.db.WithContext(ctx).Create(subscription).Error
}

func (r *blueprintSubscriptionRepo) FindByID(ctx context.Context, id uint) (*models.BlueprintSubscription, error) {
	var subscription models.BlueprintSubscription
	if err := r.db.WithContext(ctx).Where("workflow_state != ?", "deleted").First(&subscription, id).Error; err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *blueprintSubscriptionRepo) FindByTemplateAndChild(ctx context.Context, templateID, childCourseID uint) (*models.BlueprintSubscription, error) {
	var subscription models.BlueprintSubscription
	if err := r.db.WithContext(ctx).Where("blueprint_template_id = ? AND child_course_id = ? AND workflow_state != ?", templateID, childCourseID, "deleted").First(&subscription).Error; err != nil {
		return nil, err
	}
	return &subscription, nil
}

func (r *blueprintSubscriptionRepo) Update(ctx context.Context, subscription *models.BlueprintSubscription) error {
	return r.db.WithContext(ctx).Save(subscription).Error
}

func (r *blueprintSubscriptionRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.BlueprintSubscription{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *blueprintSubscriptionRepo) ListByTemplateID(ctx context.Context, templateID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	var subscriptions []models.BlueprintSubscription
	var count int64

	query := r.db.WithContext(ctx).Model(&models.BlueprintSubscription{}).Where("blueprint_template_id = ? AND workflow_state != ?", templateID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.BlueprintSubscription]{
		Items:      subscriptions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *blueprintSubscriptionRepo) ListByChildCourseID(ctx context.Context, childCourseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.BlueprintSubscription], error) {
	var subscriptions []models.BlueprintSubscription
	var count int64

	query := r.db.WithContext(ctx).Model(&models.BlueprintSubscription{}).Where("child_course_id = ? AND workflow_state != ?", childCourseID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("created_at DESC").Find(&subscriptions).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.BlueprintSubscription]{
		Items:      subscriptions,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
