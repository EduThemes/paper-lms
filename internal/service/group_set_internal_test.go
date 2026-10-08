package service

// G100: group-set planning and leadership. Example data only.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func students(ids ...uint) []GroupSetStudent {
	out := make([]GroupSetStudent, len(ids))
	for i, id := range ids {
		out[i] = GroupSetStudent{ID: id}
	}
	return out
}

func TestPlanEvenly_FillsSmallestGroupsAndHonorsLimits(t *testing.T) {
	two := 2
	groups := []models.Group{{ID: 1}, {ID: 2, MaxMembership: &two}, {ID: 3}}
	counts := map[uint]int64{1: 3, 2: 1, 3: 0}
	plan := planEvenly(groups, counts, &models.GroupCategory{}, students(10, 11, 12, 13, 14))

	total := 0
	for _, users := range plan {
		total += len(users)
	}
	assert.Equal(t, 5, total, "everyone is placed")
	assert.Len(t, plan[2], 1, "group 2 stops at its limit of 2")
	// Final sizes: group1 3+x, group2 2, group3 0+y with x+y = 4 and the
	// smallest-first rule keeps them within one of each other.
	s1, s3 := 3+len(plan[1]), len(plan[3])
	assert.LessOrEqual(t, s1-s3, 1)
	assert.LessOrEqual(t, s3-s1, 1)
}

func TestPlanEvenly_StopsWhenEveryGroupIsFull(t *testing.T) {
	one := 1
	plan := planEvenly([]models.Group{{ID: 1}}, map[uint]int64{}, &models.GroupCategory{GroupLimit: &one}, students(10, 11, 12))
	assert.Len(t, plan[1], 1)
}

func TestPlanBySection_KeepsEachGroupToOneSection(t *testing.T) {
	groups := []models.Group{{ID: 1}, {ID: 2}, {ID: 3}}
	sts := []GroupSetStudent{
		{ID: 10, SectionIDs: []uint{100}}, {ID: 11, SectionIDs: []uint{100}}, {ID: 12, SectionIDs: []uint{100}}, {ID: 13, SectionIDs: []uint{100}},
		{ID: 20, SectionIDs: []uint{200}}, {ID: 21, SectionIDs: []uint{200}},
	}
	plan, err := planBySection(groups, map[uint]int64{}, nil, sts)
	require.NoError(t, err)
	section := map[uint]uint{10: 100, 11: 100, 12: 100, 13: 100, 20: 200, 21: 200}
	placed := 0
	for gid, users := range plan {
		require.NotEmpty(t, users)
		for _, u := range users {
			assert.Equal(t, section[users[0]], section[u], "group %d mixes sections", gid)
		}
		placed += len(users)
	}
	assert.Equal(t, 6, placed)
}

func TestPlanBySection_Preconditions(t *testing.T) {
	two := 2
	_, err := planBySection([]models.Group{{ID: 1}}, map[uint]int64{1: 1}, nil, students(10))
	assert.ErrorIs(t, err, ErrGroupsNotEmptyBySection)
	_, err = planBySection([]models.Group{{ID: 1, MaxMembership: &two}}, map[uint]int64{}, nil, students(10))
	assert.ErrorIs(t, err, ErrGroupsLimitedBySection)
	// The set's group_limit is a size limit too: the by-section plan does
	// not stop at a capacity, so it would overfill the groups.
	_, err = planBySection([]models.Group{{ID: 1}}, map[uint]int64{}, &models.GroupCategory{GroupLimit: &two}, students(10, 11, 12))
	assert.ErrorIs(t, err, ErrGroupsLimitedBySection)
	_, err = planBySection([]models.Group{{ID: 1}}, map[uint]int64{}, nil, []GroupSetStudent{{ID: 10, SectionIDs: []uint{1}}, {ID: 11, SectionIDs: []uint{2}}})
	assert.ErrorIs(t, err, ErrTooFewGroupsBySection)
}

func TestCheckAutoAssign_RefusesStudentOrganizedSets(t *testing.T) {
	cid := uint(5)
	assert.NoError(t, CheckAutoAssign(&models.GroupCategory{CourseID: &cid}))
	assert.ErrorIs(t, CheckAutoAssign(&models.GroupCategory{CourseID: &cid, Role: "student_organized"}), ErrAutoAssignStudentGroups)
	assert.ErrorIs(t, CheckAutoAssign(&models.GroupCategory{}), ErrGroupSetNotInCourse)
}

func TestGroupBaseName(t *testing.T) {
	assert.Equal(t, "Project team", groupBaseName("Project teams"))
	assert.Equal(t, "Class", groupBaseName("Class"))
	assert.Equal(t, "Group", groupBaseName("  "))
}

func TestCSVSafe(t *testing.T) {
	assert.Equal(t, "'=SUM(A1)", csvSafe("=SUM(A1)"))
	assert.Equal(t, "'@cmd", csvSafe("@cmd"))
	assert.Equal(t, "Jane Doe", csvSafe("Jane Doe"))
}

func TestSyncLeadership_FirstMemberBecomesLeaderAndLeaverLosesIt(t *testing.T) {
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	svc := NewGroupService(nil, groupRepo, memRepo, nil)
	category := &models.GroupCategory{ID: 7, AutoLeader: "first"}

	memRepo.On("ListByGroupID", mock.Anything, uint(21), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{
		Items: []models.GroupMembership{{UserID: 51, WorkflowState: "accepted"}, {UserID: 52, WorkflowState: "accepted"}},
	}, nil)
	first := uint(51)
	groupRepo.On("SetLeader", mock.Anything, uint(21), &first).Return(nil).Once()
	group := &models.Group{ID: 21, GroupCategoryID: 7}
	svc.syncLeadership(context.Background(), group, category)
	require.NotNil(t, group.LeaderID)
	assert.Equal(t, uint(51), *group.LeaderID)

	// The leader leaves: leadership is revoked, then refilled from the
	// remaining members.
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(21), uint(51)).Return(nil, errCommsNotFound)
	groupRepo.On("SetLeader", mock.Anything, uint(21), (*uint)(nil)).Return(nil).Once()
	memRepo.ExpectedCalls = filterCalls(memRepo.ExpectedCalls, "ListByGroupID")
	memRepo.On("ListByGroupID", mock.Anything, uint(21), mock.Anything).Return(&repository.PaginatedResult[models.GroupMembership]{
		Items: []models.GroupMembership{{UserID: 52, WorkflowState: "accepted"}},
	}, nil)
	second := uint(52)
	groupRepo.On("SetLeader", mock.Anything, uint(21), &second).Return(nil).Once()
	svc.syncLeadership(context.Background(), group, category)
	require.NotNil(t, group.LeaderID)
	assert.Equal(t, uint(52), *group.LeaderID)
}

func TestSyncLeadership_NoAutoLeaderLeavesSpotEmpty(t *testing.T) {
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	svc := NewGroupService(nil, groupRepo, memRepo, nil)
	group := &models.Group{ID: 21}
	svc.syncLeadership(context.Background(), group, &models.GroupCategory{ID: 7})
	assert.Nil(t, group.LeaderID)
	groupRepo.AssertNotCalled(t, "SetLeader", mock.Anything, mock.Anything, mock.Anything)
}

func TestSetLeader_MustBeMember(t *testing.T) {
	groupRepo := new(mocks.MockGroupRepository)
	memRepo := new(mocks.MockGroupMembershipRepository)
	svc := NewGroupService(nil, groupRepo, memRepo, nil)
	memRepo.On("FindByGroupAndUser", mock.Anything, uint(21), uint(99)).Return(nil, errCommsNotFound)
	outsider := uint(99)
	assert.ErrorIs(t, svc.SetLeader(context.Background(), &models.Group{ID: 21}, &outsider), ErrLeaderNotMember)

	groupRepo.On("SetLeader", mock.Anything, uint(21), (*uint)(nil)).Return(nil)
	g := &models.Group{ID: 21, LeaderID: &outsider}
	require.NoError(t, svc.SetLeader(context.Background(), g, nil))
	assert.Nil(t, g.LeaderID)
}

func filterCalls(calls []*mock.Call, method string) []*mock.Call {
	out := calls[:0]
	for _, c := range calls {
		if c.Method != method {
			out = append(out, c)
		}
	}
	return out
}
