package handlers_test

// G152: GET /courses/:course_id/outcome_mastery/users/:user_id — a student
// sees only their own mastery, an observer their linked student, staff any
// student; everything else is a 404.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func seedStudentMasteryCourse(t *testing.T, g *gorm.DB) uint {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, g.Exec(`INSERT INTO accounts (id, name, workflow_state, mfa_policy, default_locale, tenant_mode, max_upload_size_mb)
		VALUES (1, 'Tenant', 'active', 'off', 'en', 'higher_ed', 500)`).Error)
	for _, id := range []uint{301, 302, 303, 304} {
		require.NoError(t, g.Exec(`INSERT INTO users (id, account_id, name, email, login_id, password_hash, role) VALUES (?, 1, ?, ?, ?, 'x', 'user')`,
			id, fmt.Sprintf("User %d", id), fmt.Sprintf("u%d@example.test", id), fmt.Sprintf("u%d@example.test", id)).Error)
	}
	course := &models.Course{AccountID: 1, Name: "Chem", CourseCode: "CH-1", WorkflowState: models.CourseAvailable}
	require.NoError(t, postgres.NewCourseRepository(g).Create(ctx, course))
	student := uint(301)
	for _, e := range []models.Enrollment{
		{UserID: 301, CourseID: course.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"},
		{UserID: 302, CourseID: course.ID, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: "active"},
		{UserID: 303, CourseID: course.ID, Type: "ObserverEnrollment", Role: "ObserverEnrollment", WorkflowState: "active", AssociatedUserID: &student},
		{UserID: 304, CourseID: course.ID, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: "active"},
	} {
		e := e
		require.NoError(t, g.Create(&e).Error)
	}
	return course.ID
}

func studentMasteryApp(g *gorm.DB, callerID uint, enrollmentType string) *fiber.App {
	svc := service.NewMasteryGradebookService(
		postgres.NewEnrollmentRepository(g), postgres.NewLearningOutcomeRepository(g), postgres.NewLearningOutcomeResultRepository(g),
		postgres.NewUserRepository(g), service.NewOutcomeProficiencyService(postgres.NewOutcomeProficiencyRepository(g)))
	h := handlers.NewOutcomeProficiencyHandler(nil, svc)
	app := testutil.SetupTestApp()
	installAuthAndPagination(app, callerID)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	app.Get("/api/v1/courses/:course_id/outcome_mastery/users/:user_id", h.StudentMastery)
	return app
}

func TestStudentMastery_Audience(t *testing.T) {
	g := contentLaneDB(t)
	courseID := seedStudentMasteryCourse(t, g)
	url := func(user string) string { return fmt.Sprintf("/api/v1/courses/%d/outcome_mastery/users/%s", courseID, user) }

	student := studentMasteryApp(g, 301, "StudentEnrollment")
	resp := testutil.MakeRequest(student, http.MethodGet, url("self"), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	assert.Equal(t, float64(301), body["user_id"])

	resp = testutil.MakeRequest(student, http.MethodGet, url("302"), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a student cannot read a classmate's mastery")

	observer := studentMasteryApp(g, 303, "ObserverEnrollment")
	resp = testutil.MakeRequest(observer, http.MethodGet, url("301"), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "observer reads the linked student")
	resp = testutil.MakeRequest(observer, http.MethodGet, url("302"), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "observer cannot read an unlinked student")
	resp = testutil.MakeRequest(observer, http.MethodGet, url("self"), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "an observer is not a student of the course")

	teacher := studentMasteryApp(g, 304, "TeacherEnrollment")
	resp = testutil.MakeRequest(teacher, http.MethodGet, url("302"), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp = testutil.MakeRequest(teacher, http.MethodGet, url("304"), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "the teacher is not a student")
}
