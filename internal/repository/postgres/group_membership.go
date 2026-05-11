package postgres

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"gorm.io/gorm"
)

type groupMembershipRepo struct {
	db *gorm.DB
}

func NewGroupMembershipRepository(db *gorm.DB) repository.GroupMembershipRepository {
	return &groupMembershipRepo{db: db}
}

func (r *groupMembershipRepo) Create(ctx context.Context, membership *models.GroupMembership) error {
	return r.db.WithContext(ctx).Create(membership).Error
}

func (r *groupMembershipRepo) FindByID(ctx context.Context, id uint) (*models.GroupMembership, error) {
	var membership models.GroupMembership
	if err := r.db.WithContext(ctx).Preload("User").First(&membership, id).Error; err != nil {
		return nil, err
	}
	return &membership, nil
}

func (r *groupMembershipRepo) Update(ctx context.Context, membership *models.GroupMembership) error {
	return r.db.WithContext(ctx).Save(membership).Error
}

func (r *groupMembershipRepo) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&models.GroupMembership{}, id).Error
}

func (r *groupMembershipRepo) ListByGroupID(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupMembership], error) {
	var memberships []models.GroupMembership
	var count int64

	query := r.db.WithContext(ctx).Model(&models.GroupMembership{}).Where("group_id = ?", groupID)
	query.Count(&count)

	offset := (params.Page - 1) * params.PerPage
	if err := query.Preload("User").Offset(offset).Limit(params.PerPage).Order("id ASC").Find(&memberships).Error; err != nil {
		return nil, err
	}

	return &repository.PaginatedResult[models.GroupMembership]{
		Items:      memberships,
		TotalCount: count,
		Page:       params.Page,
		PerPage:    params.PerPage,
	}, nil
}

func (r *groupMembershipRepo) FindByGroupAndUser(ctx context.Context, groupID, userID uint) (*models.GroupMembership, error) {
	var membership models.GroupMembership
	if err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).Preload("User").First(&membership).Error; err != nil {
		return nil, err
	}
	return &membership, nil
}

func (r *groupMembershipRepo) FindUserGroupInCategory(ctx context.Context, userID, groupCategoryID uint) (*models.Group, error) {
	var group models.Group
	if err := r.db.WithContext(ctx).
		Joins("INNER JOIN group_memberships ON group_memberships.group_id = groups.id").
		Where("group_memberships.user_id = ? AND groups.group_category_id = ? AND group_memberships.workflow_state = ? AND groups.workflow_state != ?",
			userID, groupCategoryID, "accepted", "deleted").
		First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
