package service_test

// G102 — custom role permission resolution. All data is made up.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func newRoleSvc() (*service.CustomRoleService, *mocks.MockCustomRoleRepository, *mocks.MockRoleOverrideRepository) {
	roles := new(mocks.MockCustomRoleRepository)
	overrides := new(mocks.MockRoleOverrideRepository)
	svc := service.NewCustomRoleService(roles, overrides, new(mocks.MockEnrollmentRepository))
	return svc, roles, overrides
}

func uintp(v uint) *uint { return &v }

func TestEnrollmentPermissions_BaseRoleWithoutCustomRole(t *testing.T) {
	svc, _, _ := newRoleSvc()
	perms, err := svc.EnrollmentPermissions(context.Background(), &models.Enrollment{Type: "TaEnrollment", CourseID: 4}, 1)
	require.NoError(t, err)
	assert.True(t, perms[models.PermGradeSubmissions])
	assert.False(t, perms[models.PermManageAssignments])
}

func TestEnrollmentPermissions_CustomRoleOverridesAndLocks(t *testing.T) {
	svc, roles, overrides := newRoleSvc()
	roles.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.CustomRole{
		ID: 5, AccountID: 1, BaseRoleType: "ta", WorkflowState: "active",
		Permissions: `{"grade_submissions":{"enabled":true,"locked":true},"manage_pages":{"enabled":true},"manage_files":true}`,
	}, nil)
	overrides.On("ListByRoleID", mock.Anything, uint(5)).Return([]models.RoleOverride{
		// Locked in the role: the account override can't revoke it.
		{RoleID: 5, Permission: models.PermGradeSubmissions, Enabled: false, ContextType: "Account"},
		// Account override grants analytics and locks it.
		{RoleID: 5, Permission: models.PermViewAnalytics, Enabled: true, Locked: true, ContextType: "Account"},
		// Course 4 override revokes manage_pages there.
		{RoleID: 5, Permission: models.PermManagePages, Enabled: false, ContextType: "Course", ContextID: 4},
		// ...but can't revoke the account-locked analytics.
		{RoleID: 5, Permission: models.PermViewAnalytics, Enabled: false, ContextType: "Course", ContextID: 4},
		// A different course's override doesn't apply.
		{RoleID: 5, Permission: models.PermManageQuizzes, Enabled: true, ContextType: "Course", ContextID: 99},
	}, nil)

	perms, err := svc.EnrollmentPermissions(context.Background(),
		&models.Enrollment{Type: "TaEnrollment", CourseID: 4, RoleID: uintp(5)}, 1)
	require.NoError(t, err)
	assert.True(t, perms[models.PermGradeSubmissions], "locked in role")
	assert.True(t, perms[models.PermViewAnalytics], "locked by account override")
	assert.False(t, perms[models.PermManagePages], "course override revokes")
	assert.True(t, perms[models.PermManageFiles], "shorthand bool form")
	assert.False(t, perms[models.PermManageQuizzes], "other course's override ignored")
	assert.False(t, perms[models.PermReadRoster], "not in the role's set → off (WYSIWYG with the editor)")
}

func TestEnrollmentPermissions_ForeignOrDeletedRoleFallsBackToBase(t *testing.T) {
	svc, roles, _ := newRoleSvc()
	roles.On("FindByID", mock.Anything, uint(5), uint(2)).Return(nil, errors.New("not found"))
	roles.On("FindByID", mock.Anything, uint(6), uint(1)).Return(&models.CustomRole{ID: 6, AccountID: 1, WorkflowState: "deleted", Permissions: `{"manage_assignments":true}`}, nil)

	perms, err := svc.EnrollmentPermissions(context.Background(), &models.Enrollment{Type: "StudentEnrollment", RoleID: uintp(5)}, 2)
	require.NoError(t, err)
	assert.False(t, perms[models.PermManageAssignments])
	assert.True(t, perms[models.PermSendMessages])

	perms, err = svc.EnrollmentPermissions(context.Background(), &models.Enrollment{Type: "StudentEnrollment", RoleID: uintp(6)}, 1)
	require.NoError(t, err)
	assert.False(t, perms[models.PermManageAssignments])
}

func TestResolveEnrollmentRole(t *testing.T) {
	svc, roles, _ := newRoleSvc()
	roles.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.CustomRole{ID: 5, AccountID: 1, Name: "Grader", BaseRoleType: "ta", WorkflowState: "active"}, nil)
	roles.On("FindByID", mock.Anything, uint(5), uint(2)).Return(nil, errors.New("not found"))
	roles.On("FindByID", mock.Anything, uint(8), uint(1)).Return(&models.CustomRole{ID: 8, AccountID: 1, Name: "Sub", BaseRoleType: "admin", WorkflowState: "active"}, nil)

	r, err := svc.ResolveEnrollmentRole(context.Background(), 5, 1, "TaEnrollment")
	require.NoError(t, err)
	assert.Equal(t, "Grader", r.Name)

	_, err = svc.ResolveEnrollmentRole(context.Background(), 5, 1, "StudentEnrollment")
	assert.ErrorIs(t, err, service.ErrRoleNotAssignable, "base role mismatch")
	_, err = svc.ResolveEnrollmentRole(context.Background(), 5, 2, "TaEnrollment")
	assert.ErrorIs(t, err, service.ErrRoleNotAssignable, "another tenant")
	_, err = svc.ResolveEnrollmentRole(context.Background(), 8, 1, "TeacherEnrollment")
	assert.ErrorIs(t, err, service.ErrRoleNotAssignable, "admin-based role is not a course role")
}

func TestBulkSetOverrides_RejectsForeignCourseContext(t *testing.T) {
	svc, roles, _ := newRoleSvc()
	courses := new(mocks.MockCourseRepository)
	svc.WithCourseRepo(courses)
	roles.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.CustomRole{ID: 5, AccountID: 1, WorkflowState: "active"}, nil)
	courses.On("FindByID", mock.Anything, uint(77), uint(1)).Return(nil, errors.New("not found"))

	err := svc.BulkSetOverrides(context.Background(), 5, 1, []models.RoleOverride{
		{Permission: models.PermManagePages, ContextType: "Course", ContextID: 77},
	})
	assert.Error(t, err)
}
