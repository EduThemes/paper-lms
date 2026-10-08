package handlers_test

// DB-backed tests for Student View (G076) and the unpublished-course gate
// (G077). All data is made up.

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

const svSecret = "test-secret-for-student-view-0123456789"

// svSession stamps a session: masqueradeBy != 0 makes it a masquerade.
func svSession(userID, masqueradeBy uint) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", uint(1))
		if masqueradeBy != 0 {
			c.Locals("masquerade_by", masqueradeBy)
			c.Locals("admin_account_id", uint(1))
		}
		return c.Next()
	}
}

func sessionClaims(t *testing.T, resp *http.Response) jwt.MapClaims {
	t.Helper()
	for _, ck := range resp.Cookies() {
		if ck.Name == "paper_session" {
			claims := jwt.MapClaims{}
			_, err := jwt.ParseWithClaims(ck.Value, claims, func(*jwt.Token) (any, error) { return []byte(svSecret), nil })
			require.NoError(t, err)
			return claims
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestStudentView_EnterResetAndUnpublishedCourse(t *testing.T) {
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: 50, AccountID: 1, Name: "Terry Teacher", LoginID: "terry.sv@example.test", Email: "terry.sv@example.test", PasswordHash: "x", Role: "user"},
		{ID: 51, AccountID: 1, Name: "Sam Student", LoginID: "sam.sv@example.test", Email: "sam.sv@example.test", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	require.NoError(t, g.Create(&models.Course{ID: 300, AccountID: 1, Name: "Draft Course", CourseCode: "DRAFT", WorkflowState: models.CourseUnpublished}).Error)
	for _, e := range []*models.Enrollment{
		{UserID: 50, CourseID: 300, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: 51, CourseID: 300, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}

	courseRepo := postgres.NewCourseRepository(g)
	enrollRepo := postgres.NewEnrollmentRepository(g)
	userHandler := handlers.NewUserHandler(service.NewUserService(users), svSecret, "test", service.NewTokenBlacklist(), nil, nil)
	sv := handlers.NewStudentViewHandler(service.NewStudentViewService(g), courseRepo, userHandler)
	pm := middleware.NewPermissionMiddleware(enrollRepo, users, courseRepo)
	pm.SetStudentViewCheck(sv.IsTestStudentOf)

	mount := func(session fiber.Handler) *fiber.App {
		app := testutil.SetupTestApp()
		app.Use(session)
		app.Post("/courses/:course_id/student_view", pm.RequireInstructor(), sv.Enter)
		app.Delete("/courses/:course_id/student_view", pm.RequireEnrolled(), sv.Reset)
		app.Get("/users/self/student_view", sv.Status)
		app.Get("/courses/:course_id/ping", pm.RequireEnrolled(), func(c *fiber.Ctx) error { return c.SendString("ok") })
		return app
	}

	// A real student can't enter Student View, nor read the draft course.
	student := mount(svSession(51, 0))
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodPost, "/courses/300/student_view", nil).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodGet, "/courses/300/ping", nil).StatusCode)

	// The teacher enters: the session becomes the test student, with the
	// teacher as masquerade_by.
	teacher := mount(svSession(50, 0))
	resp := testutil.MakeRequest(teacher, http.MethodPost, "/courses/300/student_view", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	claims := sessionClaims(t, resp)
	testID := uint(claims["id"].(float64))
	assert.EqualValues(t, 50, claims["masquerade_by"])
	assert.Equal(t, "user", claims["role"])
	assert.NotEqual(t, uint(50), testID)

	var testUser models.User
	require.NoError(t, g.First(&testUser, testID).Error)
	assert.Equal(t, "Test Student", testUser.Name)
	assert.True(t, strings.HasSuffix(testUser.LoginID, "@student-view.invalid"))
	assert.Error(t, testUser.CheckPassword(""), "no password can log in as the test student")

	// Entering again reuses the same test student.
	resp = testutil.MakeRequest(teacher, http.MethodPost, "/courses/300/student_view", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, testID, sessionClaims(t, resp)["id"])

	// In Student View the draft course opens; the test student alone (no
	// masquerade) does not get in.
	viewing := mount(svSession(testID, 50))
	assert.Equal(t, http.StatusOK, testutil.MakeRequest(viewing, http.MethodGet, "/courses/300/ping", nil).StatusCode)
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(mount(svSession(testID, 0)), http.MethodGet, "/courses/300/ping", nil).StatusCode)
	status, _ := testutil.ParseJSONMap(testutil.MakeRequest(viewing, http.MethodGet, "/users/self/student_view", nil))
	assert.EqualValues(t, 300, status["course_id"])
	status, _ = testutil.ParseJSONMap(testutil.MakeRequest(teacher, http.MethodGet, "/users/self/student_view", nil))
	assert.Nil(t, status["course_id"])

	// Nested entry is refused (the test student isn't course staff).
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(viewing, http.MethodPost, "/courses/300/student_view", nil).StatusCode)

	// Reset from inside Student View: a fresh test student takes over.
	resp = testutil.MakeRequest(viewing, http.MethodDelete, "/courses/300/student_view", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	fresh := sessionClaims(t, resp)
	assert.NotEqual(t, float64(testID), fresh["id"])
	assert.EqualValues(t, 50, fresh["masquerade_by"])
	var old models.Enrollment
	require.NoError(t, g.Where("user_id = ? AND course_id = ?", testID, 300).First(&old).Error)
	assert.Equal(t, models.EnrollmentDeleted, old.WorkflowState)

	// A real student can't reset it.
	assert.Equal(t, http.StatusForbidden, testutil.MakeRequest(student, http.MethodDelete, "/courses/300/student_view", nil).StatusCode)
	// The teacher can, from outside.
	resp = testutil.MakeRequest(teacher, http.MethodDelete, "/courses/300/student_view", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var markers int64
	require.NoError(t, g.Model(&models.StudentViewStudent{}).Where("course_id = ?", 300).Count(&markers).Error)
	assert.Zero(t, markers)
}
