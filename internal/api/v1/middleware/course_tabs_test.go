package middleware_test

// G078: a tab the course hides is refused server-side for learners on its
// list endpoint, and only there. All data is made up.

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCourseTabHidden(t *testing.T) {
	cfg := `[{"id":"home","hidden":false,"position":0},{"id":"pages","hidden":true,"position":1},{"id":"files","position":2}]`
	assert.True(t, middleware.CourseTabHidden(cfg, "pages"))
	assert.False(t, middleware.CourseTabHidden(cfg, "files"), "no hidden flag means visible")
	assert.False(t, middleware.CourseTabHidden(cfg, "quizzes"), "absent tab is visible")
	assert.False(t, middleware.CourseTabHidden("", "pages"))
	assert.False(t, middleware.CourseTabHidden("not json", "pages"), "an unreadable config hides nothing")
	assert.False(t, middleware.CourseTabHidden(`[{"id":"pages","hidden":"yes"}]`, "pages"))
}

func TestRequireCourseTab(t *testing.T) {
	const hiddenPages = `[{"id":"pages","hidden":true,"position":0},{"id":"files","hidden":false,"position":1}]`
	cases := []struct {
		name       string
		role       string
		enrollment *models.Enrollment
		path       string
		masquerade bool
		want       int
	}{
		{"student on a hidden tab", "user", &models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", false, http.StatusNotFound},
		{"observer on a hidden tab", "user", &models.Enrollment{Type: "ObserverEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", false, http.StatusNotFound},
		{"student view on a hidden tab", "user", &models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", true, http.StatusNotFound},
		{"student on a visible tab", "user", &models.Enrollment{Type: "StudentEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/files", false, http.StatusOK},
		{"teacher on a hidden tab", "user", &models.Enrollment{Type: "TeacherEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", false, http.StatusOK},
		{"ta on a hidden tab", "user", &models.Enrollment{Type: "TaEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", false, http.StatusOK},
		{"designer on a hidden tab", "user", &models.Enrollment{Type: "DesignerEnrollment", WorkflowState: "active"}, "/api/v1/courses/101/pages", false, http.StatusOK},
		{"admin on a hidden tab", "admin", nil, "/api/v1/courses/101/pages", false, http.StatusOK},
		// Not enrolled: the gate steps aside and the route's own guard
		// (stubbed as ok here) decides.
		{"not enrolled falls through", "user", nil, "/api/v1/courses/101/pages", false, http.StatusOK},
		{"concluded enrollment falls through", "user", &models.Enrollment{Type: "StudentEnrollment", WorkflowState: "completed"}, "/api/v1/courses/101/pages", false, http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newPermFixture()
			course := &models.Course{ID: 101, AccountID: 1, WorkflowState: models.CourseAvailable, NavigationTabs: hiddenPages}
			f.courses.On("FindByID", mock.Anything, uint(101), uint(1)).Return(course, nil)
			f.user(3, 1, tc.role)
			if tc.masquerade {
				f.user(9, 1, "user")
			}
			if tc.enrollment != nil {
				f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(3), uint(101), uint(1)).Return(tc.enrollment, nil)
			} else {
				f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(3), uint(101), uint(1)).Return(nil, errMissing)
			}

			extra := map[string]interface{}{}
			if tc.masquerade {
				extra["masquerade_by"] = uint(9)
			}
			app := testutil.SetupTestApp()
			app.Use(stub(3, 1, extra))
			// The gate is its own route ahead of the real one, as in
			// routes_pages_files.go.
			app.Get("/api/v1/courses/:course_id/pages", f.pm.RequireCourseTab(middleware.CourseTabPages))
			app.Get("/api/v1/courses/:course_id/pages", ok)
			app.Get("/api/v1/courses/:course_id/files", f.pm.RequireCourseTab(middleware.CourseTabFiles))
			app.Get("/api/v1/courses/:course_id/files", ok)

			assert.Equal(t, tc.want, testutil.MakeRequest(app, http.MethodGet, tc.path, nil).StatusCode)
		})
	}
}

// A course in another tenant is not the gate's call: it falls through and
// the real route gate answers 404.
func TestRequireCourseTab_OtherTenantFallsThrough(t *testing.T) {
	f := newPermFixture()
	f.courses.On("FindByID", mock.Anything, uint(101), uint(2)).Return(nil, errMissing)
	f.user(3, 2, "user")

	app := testutil.SetupTestApp()
	app.Use(stub(3, 2, nil))
	app.Get("/api/v1/courses/:course_id/pages", f.pm.RequireCourseTab(middleware.CourseTabPages))
	app.Get("/api/v1/courses/:course_id/pages", ok)

	assert.Equal(t, http.StatusOK, testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/pages", nil).StatusCode)
}

// A course folder's file / subfolder listings are the Files index too:
// the folder resolves to its course and the hidden tab applies. A folder
// that is not course content (user / group folders) is not the gate's
// call.
func TestRequireCourseTabFor_FolderListings(t *testing.T) {
	const hiddenFiles = `[{"id":"files","hidden":true,"position":0}]`
	folderCourse := func(c *fiber.Ctx) (uint, bool) {
		if c.Params("folder_id") == "7" {
			return 101, true
		}
		return 0, false // a user folder
	}
	run := func(enrollment string, path string) int {
		f := newPermFixture()
		f.courses.On("FindByID", mock.Anything, uint(101), uint(1)).Return(&models.Course{ID: 101, AccountID: 1, WorkflowState: models.CourseAvailable, NavigationTabs: hiddenFiles}, nil)
		f.user(3, 1, "user")
		f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(3), uint(101), uint(1)).Return(&models.Enrollment{Type: enrollment, WorkflowState: "active"}, nil)
		app := testutil.SetupTestApp()
		app.Use(stub(3, 1, nil))
		gate := f.pm.RequireCourseTabFor(middleware.CourseTabFiles, folderCourse)
		app.Get("/api/v1/folders/:folder_id/files", gate)
		app.Get("/api/v1/folders/:folder_id/files", ok)
		app.Get("/api/v1/folders/:folder_id/folders", gate)
		app.Get("/api/v1/folders/:folder_id/folders", ok)
		return testutil.MakeRequest(app, http.MethodGet, path, nil).StatusCode
	}
	assert.Equal(t, http.StatusNotFound, run("StudentEnrollment", "/api/v1/folders/7/files"))
	assert.Equal(t, http.StatusNotFound, run("StudentEnrollment", "/api/v1/folders/7/folders"))
	assert.Equal(t, http.StatusOK, run("TeacherEnrollment", "/api/v1/folders/7/files"))
	assert.Equal(t, http.StatusOK, run("StudentEnrollment", "/api/v1/folders/8/files"), "not a course folder")
}
