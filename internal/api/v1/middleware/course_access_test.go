package middleware_test

// G106: a concluded course (or enrollment) is read-only for everyone but
// admins, on the mock-repository fallback path too. All data is made up.

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCourseGate_ConcludedIsReadOnly(t *testing.T) {
	cases := []struct {
		name       string
		course     models.CourseWorkflow
		enrollment models.EnrollmentWorkflow
		typ        string
		method     string
		want       int
	}{
		{"completed course, student reads", models.CourseCompleted, models.EnrollmentActive, "StudentEnrollment", http.MethodGet, http.StatusOK},
		{"completed course, student writes", models.CourseCompleted, models.EnrollmentActive, "StudentEnrollment", http.MethodPost, http.StatusForbidden},
		{"completed course, teacher writes", models.CourseCompleted, models.EnrollmentActive, "TeacherEnrollment", http.MethodPut, http.StatusForbidden},
		{"concluded enrollment, student reads", models.CourseAvailable, models.EnrollmentCompleted, "StudentEnrollment", http.MethodGet, http.StatusOK},
		{"concluded enrollment, student writes", models.CourseAvailable, models.EnrollmentCompleted, "StudentEnrollment", http.MethodDelete, http.StatusForbidden},
		{"inactive enrollment reads", models.CourseAvailable, models.EnrollmentInactive, "StudentEnrollment", http.MethodGet, http.StatusForbidden},
		{"active enrollment writes", models.CourseAvailable, models.EnrollmentActive, "StudentEnrollment", http.MethodPost, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPermFixture()
			f.course(tc.course)
			f.user(7, 1, "user")
			f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(7), uint(101), uint(1)).
				Return(&models.Enrollment{ID: 1, UserID: 7, CourseID: 101, Type: tc.typ, WorkflowState: tc.enrollment}, nil)

			app := testutil.SetupTestApp()
			app.Use(stub(7, 1, nil))
			app.All("/api/v1/courses/:course_id/modules", f.pm.RequireEnrolled(), ok)

			assert.Equal(t, tc.want, testutil.MakeRequest(app, tc.method, "/api/v1/courses/101/modules", nil).StatusCode)
		})
	}
}

func TestCourseGate_AdminNeverReadOnly(t *testing.T) {
	f := newPermFixture()
	f.course(models.CourseCompleted)
	f.user(3, 1, "admin")
	app := testutil.SetupTestApp()
	app.Use(stub(3, 1, nil))
	app.Post("/api/v1/courses/:course_id/modules", f.pm.RequireEnrolled(), ok)
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodPost, "/api/v1/courses/101/modules", nil).StatusCode)
}
