package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type groupRepo struct {
	db *gorm.DB
}

func NewGroupRepository(db *gorm.DB) repository.GroupRepository {
	return &groupRepo{db: db}
}

func (r *groupRepo) Create(ctx context.Context, group *models.Group) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *groupRepo) FindByID(ctx context.Context, id, accountID uint) (*models.Group, error) {
	var group models.Group
	q := r.db.WithContext(ctx)
	if accountID != 0 {
		// Polymorphic dual-scope:
		//  - context_type='Account' → context_id IS the account_id
		//  - context_type='Course'  → context_id→courses.account_id
		q = q.Where(`
			(context_type = 'Account' AND context_id = ?)
			OR (context_type = 'Course' AND context_id IN (SELECT id FROM courses WHERE account_id = ?))
		`, accountID, accountID)
	}
	if err := q.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (r *groupRepo) Update(ctx context.Context, group *models.Group) error {
	return r.db.WithContext(ctx).Save(group).Error
}

func (r *groupRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&models.Group{}).Where("id = ?", id).Update("workflow_state", "deleted").Error
}

func (r *groupRepo) ListByCategoryID(ctx context.Context, categoryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	var groups []models.Group
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Group{}).Where("group_category_id = ? AND workflow_state != ?", categoryID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("name ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Group]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *groupRepo) ListByContextID(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	var groups []models.Group
	var count int64

	query := r.db.WithContext(ctx).Model(&models.Group{}).Where("context_type = ? AND context_id = ? AND workflow_state != ?", contextType, contextID, "deleted")
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Offset(offset).Limit(params.PerPage).Order("name ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Group]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *groupRepo) ListByUserID(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	var groups []models.Group
	var count int64

	baseQuery := r.db.WithContext(ctx).Model(&models.Group{}).
		Joins("INNER JOIN group_memberships ON group_memberships.group_id = groups.id").
		Where("group_memberships.user_id = ? AND group_memberships.workflow_state = ? AND groups.workflow_state != ?", userID, "accepted", "deleted")

	baseQuery.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := baseQuery.Offset(offset).Limit(params.PerPage).Order("groups.name ASC").Find(&groups).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.Group]{
		Items:      groups,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}
