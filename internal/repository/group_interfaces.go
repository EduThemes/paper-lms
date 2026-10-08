package repository

import (
	"context"

	"github.com/EduThemes/paper-lms/internal/domain/models"
)

// Groups

type GroupCategoryRepository interface {
	Create(ctx context.Context, category *models.GroupCategory) error
	// FindByID — 13.1.D: dual-scope. Course-level rows scope via course's
	// account_id; account-level rows scope direct on account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.GroupCategory, error)
	Update(ctx context.Context, category *models.GroupCategory) error
	Delete(ctx context.Context, id uint) error
	ListByCourseID(ctx context.Context, courseID uint, params PaginationParams) (*PaginatedResult[models.GroupCategory], error)
	ListByAccountID(ctx context.Context, accountID uint, params PaginationParams) (*PaginatedResult[models.GroupCategory], error)
}

type GroupRepository interface {
	Create(ctx context.Context, group *models.Group) error
	// FindByID — 13.1.D: dual-scope. Course-context groups scope via course's
	// account_id; account-context groups scope direct on account_id.
	FindByID(ctx context.Context, id, accountID uint) (*models.Group, error)
	Update(ctx context.Context, group *models.Group) error
	Delete(ctx context.Context, id uint) error
	ListByCategoryID(ctx context.Context, categoryID uint, params PaginationParams) (*PaginatedResult[models.Group], error)
	ListByContextID(ctx context.Context, contextType string, contextID uint, params PaginationParams) (*PaginatedResult[models.Group], error)
	ListByUserID(ctx context.Context, userID uint, params PaginationParams) (*PaginatedResult[models.Group], error)
	// SetLeader writes only groups.leader_id (nil clears it), so a
	// leadership change never clobbers a concurrent edit of the group.
	SetLeader(ctx context.Context, groupID uint, leaderID *uint) error
}

type GroupMembershipRepository interface {
	Create(ctx context.Context, membership *models.GroupMembership) error
	// FindByID — 13.1.D: tenant scope via parent group's account/course.
	FindByID(ctx context.Context, id, accountID uint) (*models.GroupMembership, error)
	Update(ctx context.Context, membership *models.GroupMembership) error
	Delete(ctx context.Context, id uint) error
	ListByGroupID(ctx context.Context, groupID uint, params PaginationParams) (*PaginatedResult[models.GroupMembership], error)
	FindByGroupAndUser(ctx context.Context, groupID, userID uint) (*models.GroupMembership, error)
	// FindUserGroupInCategory finds the group a user belongs to within a given group category.
	// Returns the group ID or an error if the user is not in any group in that category.
	FindUserGroupInCategory(ctx context.Context, userID, groupCategoryID uint) (*models.Group, error)
	// ListUserIDsInCategory returns the users with an accepted membership
	// in any non-deleted group of the set (the "assigned" users).
	ListUserIDsInCategory(ctx context.Context, groupCategoryID uint) ([]uint, error)
	// CountByGroupIDs returns the membership count of each group.
	CountByGroupIDs(ctx context.Context, groupIDs []uint) (map[uint]int64, error)
}
