package service

import (
	"context"
	"errors"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
)

type GroupService struct {
	catRepo        repository.GroupCategoryRepository
	groupRepo      repository.GroupRepository
	membershipRepo repository.GroupMembershipRepository
	enrollmentRepo repository.EnrollmentRepository
}

func NewGroupService(
	catRepo repository.GroupCategoryRepository,
	groupRepo repository.GroupRepository,
	membershipRepo repository.GroupMembershipRepository,
	enrollmentRepo repository.EnrollmentRepository,
) *GroupService {
	return &GroupService{
		catRepo:        catRepo,
		groupRepo:      groupRepo,
		membershipRepo: membershipRepo,
		enrollmentRepo: enrollmentRepo,
	}
}

// ---- Category CRUD ----

func (s *GroupService) CreateCategory(ctx context.Context, category *models.GroupCategory) error {
	if category.Name == "" {
		return errors.New("group category name is required")
	}
	if category.SelfSignup != "" && category.SelfSignup != "enabled" && category.SelfSignup != "restricted" {
		return errors.New("self_signup must be 'enabled', 'restricted', or empty")
	}
	if category.AutoLeader != "" && category.AutoLeader != "first" && category.AutoLeader != "random" {
		return errors.New("auto_leader must be 'first', 'random', or empty")
	}
	if category.WorkflowState == "" {
		category.WorkflowState = "active"
	}
	return s.catRepo.Create(ctx, category)
}

func (s *GroupService) GetCategory(ctx context.Context, id, accountID uint) (*models.GroupCategory, error) {
	return s.catRepo.FindByID(ctx, id, accountID)
}

func (s *GroupService) UpdateCategory(ctx context.Context, category *models.GroupCategory) error {
	if category.SelfSignup != "" && category.SelfSignup != "enabled" && category.SelfSignup != "restricted" {
		return errors.New("self_signup must be 'enabled', 'restricted', or empty")
	}
	if category.AutoLeader != "" && category.AutoLeader != "first" && category.AutoLeader != "random" {
		return errors.New("auto_leader must be 'first', 'random', or empty")
	}
	return s.catRepo.Update(ctx, category)
}

func (s *GroupService) DeleteCategory(ctx context.Context, id uint) error {
	return s.catRepo.Delete(ctx, id)
}

func (s *GroupService) ListCategoriesByCourse(ctx context.Context, courseID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	return s.catRepo.ListByCourseID(ctx, courseID, params)
}

func (s *GroupService) ListCategoriesByAccount(ctx context.Context, accountID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupCategory], error) {
	return s.catRepo.ListByAccountID(ctx, accountID, params)
}

// ---- Group CRUD ----

func (s *GroupService) CreateGroup(ctx context.Context, group *models.Group) error {
	if group.Name == "" {
		return errors.New("group name is required")
	}
	if group.WorkflowState == "" {
		group.WorkflowState = "available"
	}
	if group.JoinLevel == "" {
		group.JoinLevel = "invitation_only"
	}
	if group.ContextType == "" {
		group.ContextType = "Course"
	}
	// Verify category exists (internal lookup, tenant gate enforced by caller).
	_, err := s.catRepo.FindByID(ctx, group.GroupCategoryID, 0)
	if err != nil {
		return errors.New("group category not found")
	}
	return s.groupRepo.Create(ctx, group)
}

func (s *GroupService) GetGroup(ctx context.Context, id, accountID uint) (*models.Group, error) {
	return s.groupRepo.FindByID(ctx, id, accountID)
}

func (s *GroupService) UpdateGroup(ctx context.Context, group *models.Group) error {
	return s.groupRepo.Update(ctx, group)
}

func (s *GroupService) DeleteGroup(ctx context.Context, id uint) error {
	return s.groupRepo.Delete(ctx, id)
}

func (s *GroupService) ListGroupsByCategory(ctx context.Context, categoryID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	return s.groupRepo.ListByCategoryID(ctx, categoryID, params)
}

func (s *GroupService) ListGroupsByContext(ctx context.Context, contextType string, contextID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	return s.groupRepo.ListByContextID(ctx, contextType, contextID, params)
}

func (s *GroupService) ListUserGroups(ctx context.Context, userID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.Group], error) {
	return s.groupRepo.ListByUserID(ctx, userID, params)
}

// ---- Member management ----

func (s *GroupService) AddMember(ctx context.Context, membership *models.GroupMembership) error {
	if membership.GroupID == 0 {
		return errors.New("group_id is required")
	}
	if membership.UserID == 0 {
		return errors.New("user_id is required")
	}

	// Check if already a member
	existing, _ := s.membershipRepo.FindByGroupAndUser(ctx, membership.GroupID, membership.UserID)
	if existing != nil {
		return errors.New("user is already a member of this group")
	}

	// Check max_membership (internal lookup, tenant gate enforced by caller).
	group, err := s.groupRepo.FindByID(ctx, membership.GroupID, 0)
	if err != nil {
		return errors.New("group not found")
	}
	if group.MaxMembership != nil && *group.MaxMembership > 0 {
		members, err := s.membershipRepo.ListByGroupID(ctx, membership.GroupID, repository.PaginationParams{Page: 1, PerPage: 1})
		if err != nil {
			return err
		}
		if members.TotalCount >= int64(*group.MaxMembership) {
			return errors.New("group has reached its maximum membership limit")
		}
	}

	if membership.WorkflowState == "" {
		membership.WorkflowState = "accepted"
	}

	return s.membershipRepo.Create(ctx, membership)
}

func (s *GroupService) RemoveMember(ctx context.Context, id uint) error {
	return s.membershipRepo.Delete(ctx, id)
}

func (s *GroupService) UpdateMembership(ctx context.Context, membership *models.GroupMembership) error {
	return s.membershipRepo.Update(ctx, membership)
}

func (s *GroupService) ListMembers(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupMembership], error) {
	return s.membershipRepo.ListByGroupID(ctx, groupID, params)
}

func (s *GroupService) GetMembership(ctx context.Context, id, accountID uint) (*models.GroupMembership, error) {
	return s.membershipRepo.FindByID(ctx, id, accountID)
}
