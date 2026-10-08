package middleware_test

// G102 — custom role permission sets are enforced on the staff-gated
// course routes. All data is made up.

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

type fakeResolver struct {
	perms map[string]bool
	calls int
}

func (r *fakeResolver) EnrollmentPermissions(_ context.Context, _ *models.Enrollment, accountID uint) (map[string]bool, error) {
	r.calls++
	if accountID != 1 {
		return map[string]bool{}, nil
	}
	return r.perms, nil
}

func roleID(id uint) *uint { return &id }

func customRoleApp(t *testing.T, enrollment *models.Enrollment, perms map[string]bool) (*fakeResolver, func(method, path string) int) {
	t.Helper()
	f := newPermFixture()
	f.course(models.CourseAvailable)
	f.user(7, 1, "user")
	f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(101), uint(1)).Return(enrollment, nil)
	res := &fakeResolver{perms: perms}
	f.pm.SetPermissionResolver(res)

	app := testutil.SetupTestApp()
	app.Use(stub(7, 1, nil))
	instructor := f.pm.RequireInstructor()
	teacherOnly := f.pm.RequireCourseRole("TeacherEnrollment")
	app.Post("/api/v1/courses/:course_id/assignments", instructor, ok)
	app.Put("/api/v1/courses/:course_id/assignments/:assignment_id/submissions/:user_id", instructor, ok)
	app.Post("/api/v1/courses/:course_id/pages", instructor, ok)
	app.Get("/api/v1/courses/:course_id/gradebook", instructor, ok)
	app.Get("/api/v1/courses/:course_id/audit_log", teacherOnly, ok)
	app.Post("/api/v1/courses/:course_id/attendance", instructor, ok) // unmapped
	app.Get("/api/v1/courses/:course_id/modules", f.pm.RequireEnrolled(), ok)
	return res, func(method, path string) int {
		return testutil.MakeRequest(app, method, path, nil).StatusCode
	}
}

func TestCustomRole_RestrictsTeacherBasedRole(t *testing.T) {
	// A teacher-based "Grader" role: may grade and see grades, nothing else.
	e := &models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: "TeacherEnrollment", RoleID: roleID(5), WorkflowState: models.EnrollmentActive}
	_, do := customRoleApp(t, e, map[string]bool{
		models.PermGradeSubmissions: true,
		models.PermViewAllGrades:    true,
	})

	assert.Equal(t, http.StatusForbidden, do(http.MethodPost, "/api/v1/courses/101/assignments"), "manage_assignments not granted")
	assert.Equal(t, http.StatusForbidden, do(http.MethodPost, "/api/v1/courses/101/pages"), "manage_pages not granted")
	assert.Equal(t, http.StatusForbidden, do(http.MethodGet, "/api/v1/courses/101/audit_log"), "view_audit_log not granted")
	assert.Equal(t, http.StatusOK, do(http.MethodPut, "/api/v1/courses/101/assignments/3/submissions/9"), "grade_submissions granted")
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/api/v1/courses/101/gradebook"), "view_all_grades granted")
	// Routes without a permission mapping stay type-gated.
	assert.Equal(t, http.StatusOK, do(http.MethodPost, "/api/v1/courses/101/attendance"))
	// Enrolled-only routes are untouched.
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/api/v1/courses/101/modules"))
}

func TestCustomRole_GrantsStaffBasedRoleBeyondBaseType(t *testing.T) {
	// A TA-based role given view_audit_log passes the teacher-only gate.
	e := &models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: "TaEnrollment", RoleID: roleID(5), WorkflowState: models.EnrollmentActive}
	_, do := customRoleApp(t, e, map[string]bool{models.PermViewAuditLog: true})
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/api/v1/courses/101/audit_log"))
}

func TestCustomRole_NeverLiftsLearnerIntoStaffRoutes(t *testing.T) {
	// A student-based role, even with every permission, stays out of the
	// staff routes: their handlers assume a staff caller.
	all := map[string]bool{}
	for _, p := range models.AllPermissionNames() {
		all[p] = true
	}
	e := &models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: "StudentEnrollment", RoleID: roleID(5), WorkflowState: models.EnrollmentActive}
	_, do := customRoleApp(t, e, all)
	assert.Equal(t, http.StatusForbidden, do(http.MethodPost, "/api/v1/courses/101/assignments"))
	assert.Equal(t, http.StatusForbidden, do(http.MethodGet, "/api/v1/courses/101/gradebook"))
	assert.Equal(t, http.StatusOK, do(http.MethodGet, "/api/v1/courses/101/modules"))
}

func TestCustomRole_BaseEnrollmentUnaffected(t *testing.T) {
	e := &models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: "TaEnrollment", WorkflowState: models.EnrollmentActive}
	res, do := customRoleApp(t, e, map[string]bool{})
	assert.Equal(t, http.StatusOK, do(http.MethodPost, "/api/v1/courses/101/assignments"))
	assert.Equal(t, http.StatusForbidden, do(http.MethodGet, "/api/v1/courses/101/audit_log"))
	assert.Equal(t, 0, res.calls, "no custom role → resolver never consulted")
}

func TestRequiredCoursePermission(t *testing.T) {
	cases := []struct {
		method, path, want string
		mapped             bool
	}{
		{"POST", "/api/v1/courses/:course_id/quizzes", models.PermManageQuizzes, true},
		{"PUT", "/api/v1/courses/:course_id/quiz_item_banks/:id", models.PermManageQuizzes, true},
		{"POST", "/api/v1/courses/:course_id/assignments/:assignment_id/submissions/:user_id/comments", models.PermCommentOnSubmissions, true},
		{"GET", "/api/v1/courses/:course_id/assignments/:assignment_id/submissions", models.PermViewSubmissionDetails, true},
		{"PUT", "/api/v1/courses/:id", models.PermManageContent, true},
		{"DELETE", "/api/v1/courses/:id", "", false},
		{"GET", "/api/v1/courses/:course_id/quizzes", "", false},
		{"POST", "/api/v1/courses/:course_id/outcome_groups", models.PermManageOutcomes, true},
		{"GET", "/api/v1/courses/:course_id/outcome_results", models.PermViewAllGrades, true},
		{"POST", "/api/v1/users/self/pairing_codes", "", false},
	}
	for _, tc := range cases {
		got, mapped := middleware.RequiredCoursePermission(tc.method, tc.path)
		assert.Equal(t, tc.mapped, mapped, tc.method+" "+tc.path)
		assert.Equal(t, tc.want, got, tc.method+" "+tc.path)
	}
}
