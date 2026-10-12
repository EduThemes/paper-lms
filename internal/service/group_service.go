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
	// userRepo resolves student names / login ids / SIS ids for the
	// group-set roster tools (optional; see SetUserRepository).
	userRepo repository.UserRepository
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

// Group membership errors.
var (
	ErrGroupFull                 = errors.New("group has reached its maximum membership limit")
	ErrGroupAlreadyMember        = errors.New("user is already a member of this group")
	ErrSelfSignupDisabled        = errors.New("self sign-up is not enabled for this group set")
	ErrSelfSignupNotStudent      = errors.New("only students enrolled in the course can sign up for a group")
	ErrSelfSignupNoCommonSection = errors.New("you do not share a section with the other members of this group")
)

// groupCapacity is the effective member limit of a group: its own
// max_membership when set, otherwise the group set's group_limit
// (Canvas Group#full?). 0 means unlimited.
func groupCapacity(group *models.Group, category *models.GroupCategory) int {
	if group.MaxMembership != nil && *group.MaxMembership > 0 {
		return *group.MaxMembership
	}
	if category != nil && category.GroupLimit != nil && *category.GroupLimit > 0 {
		return *category.GroupLimit
	}
	return 0
}

// allowsMultipleMemberships mirrors Canvas GroupCategory#allows_multiple_memberships?:
// only student-organized sets let a user sit in several groups at once.
func allowsMultipleMemberships(category *models.GroupCategory) bool {
	return category != nil && category.Role == "student_organized"
}

// AddMember adds a user to a group, enforcing Canvas's membership rules:
//
//   - the group must not be full (max_membership, else the group set's
//     group_limit);
//   - a user belongs to at most one group per group set: joining a group
//     moves them out of the other groups of the same set (Canvas
//     GroupMembership#ensure_mutually_exclusive_membership), except in
//     student-organized sets.
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
		return ErrGroupAlreadyMember
	}

	// Internal lookups; the tenant gate is enforced by the caller.
	group, err := s.groupRepo.FindByID(ctx, membership.GroupID, 0)
	if err != nil {
		return errors.New("group not found")
	}
	category, _ := s.catRepo.FindByID(ctx, group.GroupCategoryID, 0)

	if limit := groupCapacity(group, category); limit > 0 {
		members, err := s.membershipRepo.ListByGroupID(ctx, membership.GroupID, repository.PaginationParams{Page: 1, PerPage: 1})
		if err != nil {
			return err
		}
		if members.TotalCount >= int64(limit) {
			return ErrGroupFull
		}
	}

	if membership.WorkflowState == "" {
		membership.WorkflowState = "accepted"
	}

	if !allowsMultipleMemberships(category) {
		if err := s.leaveOtherGroupsInCategory(ctx, membership.UserID, group, category); err != nil {
			return err
		}
	}

	if err := s.membershipRepo.Create(ctx, membership); err != nil {
		return err
	}
	s.syncLeadership(ctx, group, category)
	return nil
}

// leaveOtherGroupsInCategory removes the user's memberships in the other
// groups of the target group's set (and their leadership there).
func (s *GroupService) leaveOtherGroupsInCategory(ctx context.Context, userID uint, target *models.Group, category *models.GroupCategory) error {
	// Bounded: a user can only be in a handful of groups of one set.
	for i := 0; i < 100; i++ {
		other, err := s.membershipRepo.FindUserGroupInCategory(ctx, userID, target.GroupCategoryID)
		if err != nil || other == nil || other.ID == target.ID {
			return nil
		}
		m, err := s.membershipRepo.FindByGroupAndUser(ctx, other.ID, userID)
		if err != nil || m == nil {
			return nil
		}
		if err := s.membershipRepo.Delete(ctx, m.ID); err != nil {
			return err
		}
		s.syncLeadership(ctx, other, category)
	}
	return nil
}

// CheckSelfSignup decides whether a non-instructor may add THEMSELVES to
// a course group (Canvas Group#allow_self_signup?):
//
//   - the group set must have self_signup "enabled" or "restricted";
//   - the user must be an active student of the course;
//   - "restricted" sets additionally require that the user shares a
//     section with every current member of the group.
func (s *GroupService) CheckSelfSignup(ctx context.Context, group *models.Group, userID, courseID uint) error {
	// Deleted groups and group sets are soft-deleted rows the lookups
	// still return; nobody can sign up for them.
	if group.WorkflowState == "deleted" {
		return ErrSelfSignupDisabled
	}
	category, err := s.catRepo.FindByID(ctx, group.GroupCategoryID, 0)
	if err != nil || category == nil || category.WorkflowState == "deleted" {
		return ErrSelfSignupDisabled
	}
	if category.SelfSignup != "enabled" && category.SelfSignup != "restricted" {
		return ErrSelfSignupDisabled
	}
	if s.enrollmentRepo == nil {
		return ErrSelfSignupNotStudent
	}
	// accountID=0: the course id was resolved from a tenant-scoped group
	// lookup by the caller.
	mine, err := s.studentSections(ctx, userID, courseID)
	if err != nil || len(mine) == 0 {
		return ErrSelfSignupNotStudent
	}
	if category.SelfSignup != "restricted" {
		return nil
	}

	members, err := s.membershipRepo.ListByGroupID(ctx, group.ID, repository.PaginationParams{Page: 1, PerPage: 1000})
	if err != nil {
		return err
	}
	common := mine
	for _, m := range members.Items {
		if m.UserID == userID || m.WorkflowState != "accepted" {
			continue
		}
		theirs, _ := s.studentSections(ctx, m.UserID, courseID)
		next := map[uint]bool{}
		for sec := range common {
			if theirs[sec] {
				next[sec] = true
			}
		}
		common = next
		if len(common) == 0 {
			return ErrSelfSignupNoCommonSection
		}
	}
	return nil
}

// studentSections returns the sections (0 = no section) of the user's
// active student enrollments in the course.
func (s *GroupService) studentSections(ctx context.Context, userID, courseID uint) (map[uint]bool, error) {
	enrollments, err := s.enrollmentRepo.ListByUserID(ctx, userID, 0)
	if err != nil {
		return nil, err
	}
	out := map[uint]bool{}
	for _, e := range enrollments {
		if e.CourseID != courseID || e.WorkflowState != "active" || e.Type != "StudentEnrollment" {
			continue
		}
		var sec uint
		if e.CourseSectionID != nil {
			sec = *e.CourseSectionID
		}
		out[sec] = true
	}
	return out, nil
}

// RemoveMember deletes a membership; a leader who leaves loses the role
// and, with auto_leader, someone else takes it.
func (s *GroupService) RemoveMember(ctx context.Context, id uint) error {
	m, _ := s.membershipRepo.FindByID(ctx, id, 0)
	if err := s.membershipRepo.Delete(ctx, id); err != nil {
		return err
	}
	if m != nil {
		s.syncLeadershipByID(ctx, m.GroupID)
	}
	return nil
}

func (s *GroupService) UpdateMembership(ctx context.Context, membership *models.GroupMembership) error {
	if err := s.membershipRepo.Update(ctx, membership); err != nil {
		return err
	}
	s.syncLeadershipByID(ctx, membership.GroupID)
	return nil
}

func (s *GroupService) ListMembers(ctx context.Context, groupID uint, params repository.PaginationParams) (*repository.PaginatedResult[models.GroupMembership], error) {
	return s.membershipRepo.ListByGroupID(ctx, groupID, params)
}

func (s *GroupService) GetMembership(ctx context.Context, id, accountID uint) (*models.GroupMembership, error) {
	return s.membershipRepo.FindByID(ctx, id, accountID)
}
