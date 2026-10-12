package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type blueprintSubscriptionRepo struct {
	db *gorm.DB
}

func NewBlueprintSubscriptionRepository(db *gorm.DB) repository.BlueprintSubscriptionRepository {
	return &blueprintSubscriptionRepo{db: db}
}

// Create inserts the subscription, or revives the existing row for the
// same (template, child course). Delete is a soft delete that keeps the
// row under the UNIQUE idx_template_child index, so a plain INSERT for a
// previously removed child always hit a duplicate-key error and the
// course could never be re-associated (B135). The upsert flips the old
// row back to the requested state and returns its id.
func (r *blueprintSubscriptionRepo) Create(ctx context.Context, subscription *models.BlueprintSubscription) error {
	if subscription.WorkflowState == "" {
		subscription.WorkflowState = "active"
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "blueprint_template_id"}, {Name: "child_course_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"workflow_state", "updated_at"}),
		}).
		Create(subscription).Error
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
