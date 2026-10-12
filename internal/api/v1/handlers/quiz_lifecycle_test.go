package handlers_test

// Handler tests for the quiz lifecycle endpoints (quiz-engine lane):
// settings validation (G043 / G044 / G041), manual grading scoping (G042),
// event-log ownership (G057), moderation targets (G047), regrade scoping
// (G053) and the answer-key release window on review (G043).

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

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

// fakeEventRepo records events in memory.
type fakeEventRepo struct{ rows []models.QuizSubmissionEvent }

func (f *fakeEventRepo) BulkCreate(_ context.Context, ev []models.QuizSubmissionEvent) error {
	f.rows = append(f.rows, ev...)
	return nil
}
func (f *fakeEventRepo) ListBySubmissionID(_ context.Context, id uint) ([]models.QuizSubmissionEvent, error) {
	var out []models.QuizSubmissionEvent
	for _, e := range f.rows {
		if e.QuizSubmissionID == id {
			out = append(out, e)
		}
	}
	return out, nil
}

// fakeExtensionRepo records moderation upserts in memory.
type fakeExtensionRepo struct{ saved []models.QuizExtension }

func (f *fakeExtensionRepo) Upsert(_ context.Context, ext *models.QuizExtension) error {
	f.saved = append(f.saved, *ext)
	return nil
}
func (f *fakeExtensionRepo) FindByQuizAndUser(_ context.Context, _, _ uint) (*models.QuizExtension, error) {
	return nil, nil
}
func (f *fakeExtensionRepo) ListByQuizID(_ context.Context, _ uint) ([]models.QuizExtension, error) {
	return f.saved, nil
}

type lifecycleDeps struct {
	*quizDeps
	groups      *mocks.MockAssignmentGroupRepository
	enrollments *mocks.MockEnrollmentRepository
	events      *fakeEventRepo
	extensions  *fakeExtensionRepo
}

func lifecycleApp(role string, userID uint) (*fiber.App, *lifecycleDeps) {
	d := &lifecycleDeps{
		quizDeps: &quizDeps{
			quizRepo: new(mocks.MockQuizRepository),
			qRepo:    new(mocks.MockQuizQuestionRepository),
			sRepo:    new(mocks.MockQuizSubmissionRepository),
			aRepo:    new(mocks.MockQuizSubmissionAnswerRepository),
			stimRepo: new(mocks.MockQuizStimulusRepository),
		},
		groups:      new(mocks.MockAssignmentGroupRepository),
		enrollments: new(mocks.MockEnrollmentRepository),
		events:      &fakeEventRepo{},
		extensions:  &fakeExtensionRepo{},
	}
	svc := service.NewQuizService(d.quizRepo, d.qRepo, d.sRepo, d.aRepo,
		service.WithStimulusRepo(d.stimRepo),
		service.WithQuizLifecycle(service.QuizLifecycleDeps{
			AssignmentGroups: d.groups,
			Enrollments:      d.enrollments,
			Events:           d.events,
			Extensions:       d.extensions,
		}))
	qh := handlers.NewQuizHandler(d.quizRepo).SetQuizService(svc)
	qs := handlers.NewQuizSubmissionHandler(svc, nil, nil)
	lh := handlers.NewQuizLifecycleHandler(svc)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", uint(1))
		if role != "" {
			c.Locals("enrollment_type", role)
		}
		return c.Next()
	})
	app.Put("/courses/:course_id/quizzes/:id", qh.UpdateQuiz)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions", qs.GetSubmissionQuestions)
	app.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/scores", lh.UpdateScores)
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/events", lh.RecordEvents)
	app.Post("/courses/:course_id/quizzes/:quiz_id/extensions", lh.SetExtensions)
	app.Post("/courses/:course_id/quizzes/:quiz_id/questions/:question_id/regrade", lh.RegradeQuestion)
	return app, d
}

func TestQuizSettings_ValidatesLifecycleFields(t *testing.T) {
	cases := map[string]string{
		"bad scoring policy": `{"quiz":{"scoring_policy":"keep_lowest"}}`,
		"bad hide_results":   `{"quiz":{"hide_results":"sometimes"}}`,
		"hide before show":   `{"quiz":{"show_correct_answers_at":"2026-05-02T00:00:00Z","hide_correct_answers_at":"2026-05-01T00:00:00Z"}}`,
		"bad quiz type":      `{"quiz":{"quiz_type":"exam"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			app, d := lifecycleApp("TeacherEnrollment", 2)
			d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7, WorkflowState: "unpublished"}, nil)
			resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", strings.NewReader(body))
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			d.quizRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
		})
	}
}

func TestQuizSettings_SavesLifecycleFields(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	// A practice quiz: no gradebook assignment to sync.
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7, QuizType: "practice_quiz", WorkflowState: "unpublished"}, nil)
	var saved *models.Quiz
	d.quizRepo.On("Update", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		saved = args.Get(1).(*models.Quiz)
	}).Return(nil)

	body := `{"quiz":{"scoring_policy":"keep_average","show_correct_answers":true,
		"show_correct_answers_at":"2026-05-01T00:00:00Z","hide_correct_answers_at":"2026-05-08T00:00:00Z",
		"show_correct_answers_last_attempt":true,"hide_results":"until_after_last_attempt","one_time_results":true}}`
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", strings.NewReader(body))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, "keep_average", saved.ScoringPolicy)
	assert.True(t, saved.ShowCorrectAnswersLastAttempt)
	assert.True(t, saved.OneTimeResults)
	assert.Equal(t, "until_after_last_attempt", saved.HideResults)
	require.NotNil(t, saved.ShowCorrectAnswersAt)
	require.NotNil(t, saved.HideCorrectAnswersAt)

	out, _ := testutil.ParseJSONMap(resp)
	assert.Equal(t, "keep_average", out["scoring_policy"])
	assert.Equal(t, true, out["one_time_results"])
}

func TestQuizSettings_AssignmentGroupFromAnotherCourseIs404(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7, WorkflowState: "unpublished"}, nil)
	d.groups.On("FindByID", mock.Anything, uint(40), uint(1)).Return(&models.AssignmentGroup{ID: 40, CourseID: 99, WorkflowState: "available"}, nil)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", strings.NewReader(`{"quiz":{"assignment_group_id":40}}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d.quizRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestQuizScores_SubmissionFromAnotherQuizIs404(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(30)).Return(&models.QuizSubmission{ID: 30, QuizID: 6, UserID: 10, WorkflowState: "complete"}, nil)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/30/scores", strings.NewReader(`{"fudge_points":1}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestQuizScores_InProgressAttemptIs400(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(30)).Return(&models.QuizSubmission{ID: 30, QuizID: 5, UserID: 10, WorkflowState: "untaken"}, nil)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/30/scores", strings.NewReader(`{"fudge_points":1}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestQuizEvents_OnlyTheOwnerMayWrite(t *testing.T) {
	app, d := lifecycleApp("StudentEnrollment", 11)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(30)).Return(&models.QuizSubmission{ID: 30, QuizID: 5, UserID: 10, Attempt: 1, WorkflowState: "untaken"}, nil)
	body := `{"quiz_submission_events":[{"event_type":"page_blurred"}]}`
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions/30/events", strings.NewReader(body))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Empty(t, d.events.rows)

	ownerApp, od := lifecycleApp("StudentEnrollment", 10)
	od.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	od.sRepo.On("FindByID", mock.Anything, uint(30)).Return(&models.QuizSubmission{ID: 30, QuizID: 5, UserID: 10, Attempt: 1, WorkflowState: "untaken"}, nil)
	resp = testutil.MakeRequest(ownerApp, http.MethodPost, "/courses/7/quizzes/5/submissions/30/events", strings.NewReader(body))
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	require.Len(t, od.events.rows, 1)
	assert.Equal(t, "client", od.events.rows[0].Source)

	// Server-only event types are refused.
	resp = testutil.MakeRequest(ownerApp, http.MethodPost, "/courses/7/quizzes/5/submissions/30/events",
		strings.NewReader(`{"quiz_submission_events":[{"event_type":"submission_submitted"}]}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestQuizExtensions_TargetMustBeACourseStudent(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.enrollments.On("ListActiveStudentUserIDsByCourse", mock.Anything, uint(7), uint(1)).Return([]uint{10, 12}, nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/extensions",
		strings.NewReader(`{"quiz_extensions":[{"user_id":99,"extra_attempts":1}]}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// A batch is validated as a whole: an out-of-range row is a 400 and no
// row of the batch is saved (the valid first row included).
func TestQuizExtensions_BadRowInBatchSavesNothing(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.enrollments.On("ListActiveStudentUserIDsByCourse", mock.Anything, uint(7), uint(1)).Return([]uint{10, 12}, nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/extensions",
		strings.NewReader(`{"quiz_extensions":[{"user_id":10,"extra_attempts":1},{"user_id":12,"extra_time":99999}]}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Empty(t, d.extensions.saved, "no row of a rejected batch may be saved")

	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/extensions",
		strings.NewReader(`{"quiz_extensions":[{"user_id":10,"extra_attempts":1},{"user_id":12,"extra_time":30}]}`))
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Len(t, d.extensions.saved, 2)
}

func TestQuizRegrade_QuestionFromAnotherQuizIs404(t *testing.T) {
	app, d := lifecycleApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.qRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizQuestion{ID: 3, QuizID: 8}, nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/questions/3/regrade",
		strings.NewReader(`{"regrade_option":"full_credit"}`))
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// G043: before show_correct_answers_at the student's review keeps the
// answer key redacted even though the attempt is finished.
func TestQuizReview_AnswerKeyWaitsForReleaseDate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		showAt  time.Time
		wantKey bool
	}{
		{"before release", time.Now().Add(24 * time.Hour), false},
		{"after release", time.Now().Add(-24 * time.Hour), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := lifecycleApp("StudentEnrollment", 10)
			quiz := publishedQuiz(5, 7)
			showAt := tc.showAt
			quiz.ShowCorrectAnswersAt = &showAt
			d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
			sub := &models.QuizSubmission{ID: 30, QuizID: 5, UserID: 10, Attempt: 1, WorkflowState: "complete"}
			d.sRepo.On("FindByID", mock.Anything, uint(30)).Return(sub, nil)
			d.sRepo.On("ListByQuizAndUserIDs", mock.Anything, uint(5), []uint{10}).Return([]models.QuizSubmission{*sub}, nil)
			d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(
				models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey},
			), nil)

			resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/30/questions", nil)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			arr, _ := testutil.ParseJSONArray(resp)
			require.Len(t, arr, 1)
			hasKey := strings.Contains(arr[0]["answers"].(string), "weight")
			assert.Equal(t, tc.wantKey, hasKey)
		})
	}
}

// G046: the availability window is checked against the dates in the same
// request: moving both dates later is accepted, a window that ends
// before it starts is not.
func TestQuizSettings_DateWindowUsesTheNewDates(t *testing.T) {
	stored := func() *models.Quiz {
		unlock := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
		lock := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
		return &models.Quiz{ID: 5, CourseID: 7, QuizType: "practice_quiz", WorkflowState: "unpublished", UnlockAt: &unlock, LockAt: &lock}
	}

	t.Run("both dates moved later", func(t *testing.T) {
		app, d := lifecycleApp("TeacherEnrollment", 2)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(stored(), nil)
		d.quizRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
		body := `{"quiz":{"unlock_at":"2026-04-10T00:00:00Z","lock_at":"2026-04-17T00:00:00Z"}}`
		resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", strings.NewReader(body))
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("lock before unlock", func(t *testing.T) {
		app, d := lifecycleApp("TeacherEnrollment", 2)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(stored(), nil)
		body := `{"quiz":{"lock_at":"2026-02-01T00:00:00Z"}}`
		resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", strings.NewReader(body))
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		d.quizRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
	})
}
