package handlers_test

// B068 / B171 regression tests for the course enrollments endpoints.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// courseCallerStub mimics RequireEnrolled/RequireInstructor having run
// for a caller with the given course enrollment type in account 1.
func courseCallerStub(enrollmentType string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(10))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	}
}

type enrollmentFixture struct {
	enrollments *mocks.MockEnrollmentRepository
	users       *mocks.MockUserRepository
	courses     *mocks.MockCourseRepository
	sections    *mocks.MockSectionRepository
	handler     *handlers.EnrollmentHandler
}

func newEnrollmentFixture() *enrollmentFixture {
	f := &enrollmentFixture{
		enrollments: new(mocks.MockEnrollmentRepository),
		users:       new(mocks.MockUserRepository),
		courses:     new(mocks.MockCourseRepository),
		sections:    new(mocks.MockSectionRepository),
	}
	svc := service.NewEnrollmentService(f.enrollments).SetTenantRepos(f.users, f.courses, f.sections)
	f.handler = handlers.NewEnrollmentHandler(svc)
	f.courses.On("FindByID", mock.Anything, uint(7), uint(1)).Return(&models.Course{ID: 7, AccountID: 1}, nil).Maybe()
	return f
}

func TestCreateEnrollment_CrossTenantUserIs404(t *testing.T) {
	f := newEnrollmentFixture()
	// User 500 belongs to another tenant: the account-1 lookup misses.
	f.users.On("FindByID", mock.Anything, uint(500), uint(1)).Return(nil, errors.New("record not found"))

	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/enrollments", courseCallerStub("TeacherEnrollment"), f.handler.CreateEnrollment)

	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/enrollments",
		testutil.JSONBody(map[string]any{"enrollment": map[string]any{"user_id": 500, "type": "StudentEnrollment"}}))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	f.enrollments.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreateEnrollment_TACannotAddTeacher(t *testing.T) {
	f := newEnrollmentFixture()
	app := testutil.SetupTestApp()
	app.Post("/courses/:course_id/enrollments", courseCallerStub("TaEnrollment"), f.handler.CreateEnrollment)

	for _, typ := range []string{"TeacherEnrollment", "TaEnrollment", "DesignerEnrollment"} {
		resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/enrollments",
			testutil.JSONBody(map[string]any{"enrollment": map[string]any{"user_id": 5, "type": typ}}))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, typ)
	}
	f.enrollments.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	// A TA may still add a student from their own tenant.
	f.users.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.User{ID: 5, AccountID: 1}, nil)
	f.enrollments.On("FindByUserAndCourse", mock.Anything, uint(5), uint(7), uint(1)).Return(nil, errors.New("not found"))
	f.enrollments.On("Create", mock.Anything, mock.AnythingOfType("*models.Enrollment")).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/enrollments",
		testutil.JSONBody(map[string]any{"enrollment": map[string]any{"user_id": 5, "type": "StudentEnrollment"}}))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestListEnrollments_HidesContactInfoFromStudents(t *testing.T) {
	f := newEnrollmentFixture()
	roster := &repository.PaginatedResult[models.Enrollment]{
		Items: []models.Enrollment{{
			ID: 1, UserID: 5, CourseID: 7, Type: "StudentEnrollment",
			User: &models.User{ID: 5, Name: "Jane Doe", LoginID: "jane.doe", Email: "jane.doe@example.com"},
		}},
		TotalCount: 1, Page: 1, PerPage: 50,
	}
	f.enrollments.On("ListByCourseID", mock.Anything, uint(7), uint(1), mock.Anything).Return(roster, nil)

	for _, tc := range []struct {
		role    string
		wantPII bool
	}{
		{"StudentEnrollment", false},
		{"ObserverEnrollment", false},
		{"TaEnrollment", true},
		{"TeacherEnrollment", true},
	} {
		app := testutil.SetupTestApp()
		app.Get("/courses/:course_id/enrollments", courseCallerStub(tc.role), f.handler.ListEnrollments)
		resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/enrollments", nil)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		rows, err := testutil.ParseJSONArray(resp)
		assert.NoError(t, err)
		user := rows[0]["user"].(map[string]any)
		_, hasEmail := user["email"]
		_, hasLogin := user["login_id"]
		assert.Equal(t, tc.wantPII, hasEmail, "%s email visibility", tc.role)
		assert.Equal(t, tc.wantPII, hasLogin, "%s login_id visibility", tc.role)
		assert.Equal(t, "Jane Doe", user["name"])
	}
}
