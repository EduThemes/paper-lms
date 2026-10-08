package handlers_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

// B087: outcome_results / outcome_rollups are mounted with `enrolled`, so the
// handler itself must stop a student from reading classmates' mastery data.

func outcomeAuthzApp(t *testing.T, callerID uint, enrollmentType string, enrollmentRepo *mocks.MockEnrollmentRepository) *fiber.App {
	t.Helper()
	resultRepo := newFakeLearningOutcomeResultRepo()
	// Student 42 (the caller in most cases) and classmate 43.
	resultRepo.byUser[42] = []models.LearningOutcomeResult{{ID: 1, UserID: 42, LearningOutcomeID: 5, ContextType: "Course", ContextID: 101}}
	resultRepo.byUser[43] = []models.LearningOutcomeResult{{ID: 2, UserID: 43, LearningOutcomeID: 5, ContextType: "Course", ContextID: 101}}
	score := 3.0
	resultRepo.byOutcome[5] = []models.LearningOutcomeResult{
		{ID: 1, UserID: 42, LearningOutcomeID: 5, Score: &score},
		{ID: 2, UserID: 43, LearningOutcomeID: 5, Score: &score},
	}
	outcomeRepo := &fakeLearningOutcomeRepo{byCourse: map[uint][]models.LearningOutcome{
		101: {{ID: 5, ContextType: "Course", ContextID: 101, PointsPossible: 5, MasteryPoints: 3}},
	}}
	svc := service.NewLearningOutcomeService(&fakeLearningOutcomeGroupRepo{}, outcomeRepo, resultRepo)
	h := handlers.NewLearningOutcomeHandler(svc, new(mocks.MockOutcomeAlignmentRepository), nil)
	if enrollmentRepo != nil {
		h.SetEnrollmentRepo(enrollmentRepo)
	}

	app := testutil.SetupTestApp()
	installAuthAndPagination(app, callerID)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("enrollment_type", enrollmentType)
		return c.Next()
	})
	app.Get("/api/v1/courses/:course_id/outcome_results", h.ListResults)
	app.Get("/api/v1/courses/:course_id/outcome_rollups", h.GetMasteryGradebook)
	return app
}

func outcomeResultUserIDs(t *testing.T, resp *http.Response) []float64 {
	t.Helper()
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	list, _ := body["outcome_results"].([]interface{})
	var ids []float64
	for _, it := range list {
		ids = append(ids, it.(map[string]interface{})["user_id"].(float64))
	}
	return ids
}

func rollupUserIDs(t *testing.T, resp *http.Response) []float64 {
	t.Helper()
	body, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	list, _ := body["rollups"].([]interface{})
	var ids []float64
	for _, it := range list {
		links := it.(map[string]interface{})["links"].(map[string]interface{})
		ids = append(ids, links["user"].(float64))
	}
	return ids
}

func TestListOutcomeResults_StudentCannotReadClassmate(t *testing.T) {
	app := outcomeAuthzApp(t, 42, "StudentEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results?user_id=43", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestListOutcomeResults_StudentBulkReadReturnsOnlyOwn(t *testing.T) {
	app := outcomeAuthzApp(t, 42, "StudentEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []float64{42}, outcomeResultUserIDs(t, resp))
}

func TestListOutcomeResults_StudentCanReadSelf(t *testing.T) {
	app := outcomeAuthzApp(t, 42, "StudentEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results?user_id=42", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []float64{42}, outcomeResultUserIDs(t, resp))
}

func TestListOutcomeResults_TeacherSeesEveryone(t *testing.T) {
	app := outcomeAuthzApp(t, 7, "TeacherEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.ElementsMatch(t, []float64{42, 43}, outcomeResultUserIDs(t, resp))
}

func TestOutcomeRollups_StudentSeesOnlyOwnRollup(t *testing.T) {
	app := outcomeAuthzApp(t, 42, "StudentEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_rollups", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []float64{42}, rollupUserIDs(t, resp))
}

func TestOutcomeRollups_TeacherSeesEveryone(t *testing.T) {
	app := outcomeAuthzApp(t, 7, "TeacherEnrollment", nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_rollups", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.ElementsMatch(t, []float64{42, 43}, rollupUserIDs(t, resp))
}

func TestOutcomeResults_ObserverSeesLinkedStudentOnly(t *testing.T) {
	linked := uint(42)
	enrollments := new(mocks.MockEnrollmentRepository)
	enrollments.On("ListByUserID", mock.Anything, uint(900), uint(1)).Return([]models.Enrollment{
		{ID: 1, UserID: 900, CourseID: 101, Type: "ObserverEnrollment", WorkflowState: models.EnrollmentActive, AssociatedUserID: &linked},
	}, nil)
	app := outcomeAuthzApp(t, 900, "ObserverEnrollment", enrollments)

	resp := testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results?user_id=42", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []float64{42}, outcomeResultUserIDs(t, resp))

	resp = testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_results?user_id=43", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	resp = testutil.MakeRequest(app, http.MethodGet, "/api/v1/courses/101/outcome_rollups", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []float64{42}, rollupUserIDs(t, resp))
}
