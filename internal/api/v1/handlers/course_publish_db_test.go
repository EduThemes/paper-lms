package handlers_test

// DB-backed tests for course publish gating (G077): a published course
// with graded student work can't be unpublished, while the Student View
// test student's grades never block it. All data is made up.

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/api/v1/middleware"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/repository/postgres"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
)

func TestCoursePublish_UnpublishBlockedByGradedWork(t *testing.T) {
	g := coreScratchDB(t)
	ctx := context.Background()
	users := postgres.NewUserRepository(g)
	for _, u := range []*models.User{
		{ID: 60, AccountID: 1, Name: "Tia Teacher", LoginID: "tia.pub@example.test", Email: "tia.pub@example.test", PasswordHash: "x", Role: "user"},
		{ID: 61, AccountID: 1, Name: "Sid Student", LoginID: "sid.pub@example.test", Email: "sid.pub@example.test", PasswordHash: "x", Role: "user"},
		{ID: 62, AccountID: 1, Name: "Test Student", LoginID: "test_student_pub@student-view.invalid", Email: "test_student_pub@student-view.invalid", PasswordHash: "x", Role: "user"},
	} {
		require.NoError(t, users.Create(ctx, u))
	}
	require.NoError(t, g.Create(&models.Course{ID: 310, AccountID: 1, Name: "Live Course", CourseCode: "LIVE", WorkflowState: models.CourseAvailable}).Error)
	for _, e := range []*models.Enrollment{
		{UserID: 60, CourseID: 310, Type: "TeacherEnrollment", Role: "TeacherEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: 61, CourseID: 310, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
		{UserID: 62, CourseID: 310, Type: "StudentEnrollment", Role: "StudentEnrollment", WorkflowState: models.EnrollmentActive},
	} {
		require.NoError(t, g.Create(e).Error)
	}
	require.NoError(t, g.Create(&models.StudentViewStudent{CourseID: 310, UserID: 62}).Error)
	require.NoError(t, g.Create(&models.Assignment{ID: 3101, CourseID: 310, Name: "Lab 1", WorkflowState: models.AssignmentPublished}).Error)

	courseRepo := postgres.NewCourseRepository(g)
	enrollRepo := postgres.NewEnrollmentRepository(g)
	pm := middleware.NewPermissionMiddleware(enrollRepo, users, courseRepo)
	publish := service.NewCoursePublishService(g)
	h := handlers.NewCourseHandler(
		service.NewCourseService(courseRepo, enrollRepo, postgres.NewSectionRepository(g)),
		service.NewEnrollmentService(enrollRepo),
	).SetUserRepo(users).SetUnpublishGuard(publish.HasGradedStudentWork)

	app := testutil.SetupTestApp()
	app.Use(svSession(60, 0))
	app.Put("/api/v1/courses/:id", pm.RequireInstructor(), h.UpdateCourse)

	setState := func(state string) *http.Response {
		return testutil.MakeRequest(app, http.MethodPut, "/api/v1/courses/310",
			testutil.JSONBody(map[string]any{"course": map[string]any{"workflow_state": state}}))
	}

	// Only the test student has a grade, and an ungraded submission from
	// the real student doesn't count: unpublishing is allowed.
	score := 9.0
	require.NoError(t, g.Create(&models.Submission{AssignmentID: 3101, UserID: 62, Score: &score, WorkflowState: models.SubmissionGraded}).Error)
	real := &models.Submission{AssignmentID: 3101, UserID: 61, WorkflowState: models.SubmissionSubmitted}
	require.NoError(t, g.Create(real).Error)
	graded, err := publish.HasGradedStudentWork(ctx, 310)
	require.NoError(t, err)
	assert.False(t, graded)
	require.Equal(t, http.StatusOK, setState("unpublished").StatusCode)
	require.Equal(t, http.StatusOK, setState("available").StatusCode)

	// Once a real student is graded the course stays published.
	require.NoError(t, g.Model(real).Updates(map[string]any{"score": 7.5, "workflow_state": "graded"}).Error)
	resp := setState("unpublished")
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body, _ := testutil.ParseJSONMap(resp)
	assert.True(t, strings.Contains(strings.ToLower(jsonErrorText(body)), "graded"), "explains the refusal: %v", body)
	var course models.Course
	require.NoError(t, g.First(&course, 310).Error)
	assert.Equal(t, models.CourseAvailable, course.WorkflowState)

	// Concluding is still allowed; the guard only covers unpublish.
	require.Equal(t, http.StatusOK, setState("completed").StatusCode)

	// A deleted assignment's grades don't pin the course.
	require.NoError(t, g.Model(&course).Update("workflow_state", models.CourseAvailable).Error)
	require.NoError(t, g.Model(&models.Assignment{ID: 3101}).Update("workflow_state", "deleted").Error)
	graded, err = publish.HasGradedStudentWork(ctx, 310)
	require.NoError(t, err)
	assert.False(t, graded)
}

// jsonErrorText flattens a Canvas-style error body for assertions.
func jsonErrorText(body map[string]any) string {
	var b strings.Builder
	if m, ok := body["message"].(string); ok {
		b.WriteString(m)
	}
	if errs, ok := body["errors"].([]any); ok {
		for _, e := range errs {
			if em, ok := e.(map[string]any); ok {
				if m, ok := em["message"].(string); ok {
					b.WriteString(" " + m)
				}
			}
		}
	}
	return b.String()
}
