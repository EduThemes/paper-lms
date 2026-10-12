package handlers_test

// Handler-level regression tests for the quizzes lane (Canvas parity audit):
// answer-key redaction (B023/B008), structured answers (B024), question
// membership (B025), statistics scoping (B031), staff answer review (B033),
// student attempt history (B034), stimulus embedding/linking (B126),
// unpublished quizzes (B128) and nullable PATCH fields (B129).

import (
	"context"
	"errors"
	"io"
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
	"github.com/EduThemes/paper-lms/internal/repository"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type quizDeps struct {
	quizRepo *mocks.MockQuizRepository
	qRepo    *mocks.MockQuizQuestionRepository
	sRepo    *mocks.MockQuizSubmissionRepository
	aRepo    *mocks.MockQuizSubmissionAnswerRepository
	stimRepo *mocks.MockQuizStimulusRepository
}

// quizApp mounts the quiz handlers behind a stub auth layer. role is the
// course enrollment type ("" for none); userID the caller.
func quizApp(role string, userID uint) (*fiber.App, *quizDeps) {
	d := &quizDeps{
		quizRepo: new(mocks.MockQuizRepository),
		qRepo:    new(mocks.MockQuizQuestionRepository),
		sRepo:    new(mocks.MockQuizSubmissionRepository),
		aRepo:    new(mocks.MockQuizSubmissionAnswerRepository),
		stimRepo: new(mocks.MockQuizStimulusRepository),
	}
	svc := service.NewQuizService(d.quizRepo, d.qRepo, d.sRepo, d.aRepo, service.WithStimulusRepo(d.stimRepo))
	qh := handlers.NewQuizHandler(d.quizRepo)
	qq := handlers.NewQuizQuestionHandler(svc)
	qs := handlers.NewQuizSubmissionHandler(svc, nil, nil)
	st := handlers.NewQuizStatisticsHandler(svc)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", uint(1))
		if role != "" {
			c.Locals("enrollment_type", role)
		}
		return c.Next()
	})
	app.Get("/courses/:course_id/quizzes", qh.ListQuizzes)
	app.Get("/courses/:course_id/quizzes/:id", qh.GetQuiz)
	app.Put("/courses/:course_id/quizzes/:id", qh.UpdateQuiz)
	app.Get("/courses/:course_id/quizzes/:quiz_id/questions", qq.ListQuestions)
	app.Put("/courses/:course_id/quizzes/:quiz_id/questions/:question_id", qq.UpdateQuestion)
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions", qs.StartSubmission)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions", qs.ListSubmissions)
	app.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id", qs.AnswerQuestion)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers", qs.GetSubmissionAnswers)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions", qs.GetSubmissionQuestions)
	app.Get("/courses/:course_id/quizzes/:quiz_id/statistics", st.GetQuizStatistics)
	return app, d
}

func publishedQuiz(id, courseID uint) *models.Quiz {
	return &models.Quiz{ID: id, CourseID: courseID, Published: true, WorkflowState: "published", ShowCorrectAnswers: true, AllowedAttempts: -1}
}

const mcKey = `[{"id":"a","text":"4","weight":100,"comments":"yes"},{"id":"b","text":"5","weight":0}]`

func questionPage(qs ...models.QuizQuestion) *repository.PaginatedResult[models.QuizQuestion] {
	return &repository.PaginatedResult[models.QuizQuestion]{Items: qs, TotalCount: int64(len(qs)), Page: 1, PerPage: 50}
}

// ---- B023: answer keys are redacted for students ----

func TestQuizQuestions_StudentSeesNoAnswerKey(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(
		models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey, CorrectComments: "It is 4"},
		models.QuizQuestion{ID: 2, QuizID: 5, QuestionType: "short_answer", Answers: `[{"id":"s","text":"Paris","weight":100}]`},
	), nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/questions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	require.Len(t, arr, 2)
	mc := arr[0]["answers"].(string)
	assert.NotContains(t, mc, "weight")
	assert.Contains(t, mc, `"text":"4"`)
	assert.Equal(t, "", arr[0]["correct_comments"])
	assert.NotContains(t, arr[1]["answers"].(string), "Paris")
}

func TestQuizQuestions_TeacherSeesAnswerKey(t *testing.T) {
	app, d := quizApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7, WorkflowState: "unpublished"}, nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(
		models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey},
	), nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/questions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Equal(t, mcKey, arr[0]["answers"])
}

// In-progress attempt questions are redacted; after completion the key is
// shown when the quiz allows it (show_correct_answers).
func TestSubmissionQuestions_KeyOnlyAfterCompletion(t *testing.T) {
	for _, tc := range []struct {
		state   string
		wantKey bool
	}{{"untaken", false}, {"complete", true}} {
		app, d := quizApp("StudentEnrollment", 10)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
		d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: tc.state, SelectedQuestions: "[1]"}, nil)
		d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey}, nil)

		resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions", nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		arr, _ := testutil.ParseJSONArray(resp)
		require.Len(t, arr, 1)
		assert.Equal(t, tc.wantKey, strings.Contains(arr[0]["answers"].(string), "weight"), "state %s", tc.state)
	}
}

// ---- B024 / B025: structured answers saved; foreign questions refused ----

func TestAnswerQuestion_ArrayAnswerIsStoredAsJSON(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_answer"}, nil)
	d.aRepo.On("FindBySubmissionAndQuestion", mock.Anything, uint(3), uint(1)).Return(nil, errors.New("none"))
	var stored []string
	d.aRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizSubmissionAnswer")).
		Run(func(a mock.Arguments) { stored = append(stored, a.Get(1).(*models.QuizSubmissionAnswer).Answer) }).
		Return(nil)

	// Arrays (multiple_answer / ordering) …
	body := testutil.JSONBody(map[string]interface{}{"answer": []string{"a1", "a2"}})
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/1", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// … objects (hot spot / dropdowns / categorization) …
	body = testutil.JSONBody(map[string]interface{}{"answer": map[string]float64{"x": 0.5, "y": 0.25}})
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/1", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// … and plain strings keep working.
	body = testutil.JSONBody(map[string]interface{}{"answer": "a1"})
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/1", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Equal(t, []string{`["a1","a2"]`, `{"x":0.5,"y":0.25}`, "a1"}, stored)
}

func TestAnswerQuestion_ForeignQuestion404(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(77)).Return(&models.QuizQuestion{ID: 77, QuizID: 6}, nil)

	body := testutil.JSONBody(map[string]interface{}{"answer": "a"})
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/77", body)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d.aRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ---- B031: statistics scoped to course + tenant ----

func TestQuizStatistics_CrossCourseAndCrossTenant404(t *testing.T) {
	app, d := quizApp("TeacherEnrollment", 2)
	// Quiz 5 lives in course 99, not the URL's course 7.
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 99}, nil)
	// Quiz 6 is not in the caller's tenant (tenant-scoped lookup misses).
	d.quizRepo.On("FindByID", mock.Anything, uint(6), uint(1)).Return(nil, errors.New("not found"))

	for _, path := range []string{"/courses/7/quizzes/5/statistics", "/courses/7/quizzes/6/statistics"} {
		resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
	}
	d.qRepo.AssertNotCalled(t, "ListByQuizID", mock.Anything, mock.Anything, mock.Anything)
}

// ---- B033: teachers can read a student's answers; other students can't ----

func TestSubmissionAnswers_TeacherAllowedOtherStudent404(t *testing.T) {
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "complete"}

	app, d := quizApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.aRepo.On("ListBySubmissionID", mock.Anything, uint(3)).Return([]models.QuizSubmissionAnswer{{ID: 1, QuestionID: 1, Answer: "a"}}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	app2, d2 := quizApp("StudentEnrollment", 11)
	d2.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d2.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	resp = testutil.MakeRequest(app2, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d2.aRepo.AssertNotCalled(t, "ListBySubmissionID", mock.Anything, mock.Anything)
}

// ---- B034: students list only their own attempts ----

func TestListSubmissions_StudentGetsOwnOnly(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("ListByQuizAndUserIDs", mock.Anything, uint(5), []uint{10}).Return([]models.QuizSubmission{
		{ID: 1, QuizID: 5, UserID: 10, Attempt: 1, WorkflowState: "complete"},
	}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	out, _ := testutil.ParseJSONMap(resp)
	subs := out["quiz_submissions"].([]interface{})
	require.Len(t, subs, 1)
	d.sRepo.AssertNotCalled(t, "ListByQuizID", mock.Anything, mock.Anything, mock.Anything)
}

// ---- B128: students never see or take unpublished quizzes ----

func TestUnpublishedQuiz_HiddenFromStudents(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	draft := &models.Quiz{ID: 5, CourseID: 7, WorkflowState: "unpublished"}
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(draft, nil)
	d.quizRepo.On("ListPublishedByCourseID", mock.Anything, uint(7), mock.Anything).Return(&repository.PaginatedResult[models.Quiz]{Page: 1, PerPage: 10}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/questions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	d.quizRepo.AssertNotCalled(t, "ListByCourseID", mock.Anything, mock.Anything, mock.Anything)
	d.sRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// ---- B129: explicit null clears time_limit / group ----

func TestUpdateQuiz_NullTimeLimitClears(t *testing.T) {
	app, d := quizApp("TeacherEnrollment", 2)
	limit := 30
	q := &models.Quiz{ID: 5, CourseID: 7, WorkflowState: "published", Published: true, TimeLimit: &limit}
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.quizRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *models.Quiz) bool { return u.TimeLimit == nil })).Return(nil)

	body := strings.NewReader(`{"quiz":{"time_limit":null}}`)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_, _ = io.ReadAll(resp.Body)
	// NB: no AssertExpectations between requests — formatting the recorded
	// fiber ctx corrupts the pooled RequestCtx for the next request.

	// Absent key leaves it unchanged.
	limit2 := 45
	q2 := &models.Quiz{ID: 6, CourseID: 7, WorkflowState: "published", Published: true, TimeLimit: &limit2}
	d.quizRepo.On("FindByID", mock.Anything, uint(6), uint(1)).Return(q2, nil)
	d.quizRepo.On("Update", mock.Anything, mock.MatchedBy(func(u *models.Quiz) bool { return u.ID == 6 && u.TimeLimit != nil && *u.TimeLimit == 45 })).Return(nil)
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/6", strings.NewReader(`{"quiz":{"title":"x"}}`))
	dbg, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(dbg))
}

func TestUpdateQuestion_NullGroupUngroupsAndStimulusLinks(t *testing.T) {
	app, d := quizApp("TeacherEnrollment", 2)
	gid := uint(4)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuizQuestionGroupID: &gid}, nil)
	d.stimRepo.On("FindByID", mock.Anything, uint(9)).Return(&models.QuizStimulus{ID: 9, CourseID: 7, Title: "Passage"}, nil)
	d.qRepo.On("Update", mock.Anything, mock.MatchedBy(func(q *models.QuizQuestion) bool {
		return q.QuizQuestionGroupID == nil && q.StimulusID != nil && *q.StimulusID == 9
	})).Return(nil)

	body := strings.NewReader(`{"question":{"quiz_question_group_id":null,"stimulus_id":9}}`)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/questions/1", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	d.qRepo.AssertExpectations(t)
}

// B126: a stimulus from another course cannot be linked.
func TestUpdateQuestion_ForeignStimulusRejected(t *testing.T) {
	app, d := quizApp("TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5}, nil)
	d.stimRepo.On("FindByID", mock.Anything, uint(9)).Return(&models.QuizStimulus{ID: 9, CourseID: 99}, nil)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/questions/1", strings.NewReader(`{"question":{"stimulus_id":9}}`))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	d.qRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// B126: the linked passage is embedded for the take page.
func TestQuizQuestions_EmbedStimulus(t *testing.T) {
	app, d := quizApp("StudentEnrollment", 10)
	sid := uint(9)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(
		models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "essay", StimulusID: &sid},
	), nil)
	d.stimRepo.On("FindByID", mock.Anything, uint(9)).Return(&models.QuizStimulus{ID: 9, CourseID: 7, Title: "Passage", Content: "<p>Read me</p>", CreatedAt: time.Now()}, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/questions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	stim, ok := arr[0]["stimulus"].(map[string]interface{})
	require.True(t, ok, "stimulus not embedded: %v", arr[0])
	assert.Equal(t, "Passage", stim["title"])
}

// ---- B008: question-bank entries are staff-only ----

func TestQuestionBankQuestions_StudentRefused(t *testing.T) {
	bankRepo := new(mockQuestionBankRepoNoop)
	svc := service.NewQuestionBankService(bankRepo, nil, nil, nil)
	h := handlers.NewQuestionBankHandler(svc)
	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(10))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", "StudentEnrollment")
		return c.Next()
	})
	app.Get("/courses/:course_id/question_banks/:bank_id/questions", h.ListQuestions)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/question_banks/1/questions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.False(t, bankRepo.called, "bank must not even be loaded for a student")
}

type mockQuestionBankRepoNoop struct{ called bool }

func (m *mockQuestionBankRepoNoop) Create(_ context.Context, _ *models.QuestionBank) error {
	m.called = true
	return nil
}
func (m *mockQuestionBankRepoNoop) FindByID(_ context.Context, _ uint) (*models.QuestionBank, error) {
	m.called = true
	return &models.QuestionBank{ID: 1, CourseID: 7}, nil
}
func (m *mockQuestionBankRepoNoop) Update(_ context.Context, _ *models.QuestionBank) error {
	m.called = true
	return nil
}
func (m *mockQuestionBankRepoNoop) Delete(_ context.Context, _ uint) error {
	m.called = true
	return nil
}
func (m *mockQuestionBankRepoNoop) ListByCourse(_ context.Context, _ uint, _ repository.PaginationParams) (*repository.PaginatedResult[models.QuestionBank], error) {
	m.called = true
	return &repository.PaginatedResult[models.QuestionBank]{}, nil
}

// Leftover (quizzes lane): with hide_results = "always" the student's own
// answers come back without per-answer correctness or points; the
// teacher still sees them.
func TestSubmissionAnswers_HideResultsAlwaysStripsCorrectness(t *testing.T) {
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "complete", Attempt: 1}
	quiz := publishedQuiz(5, 7)
	quiz.HideResults = "always"
	correct, pts := true, 2.0
	answers := []models.QuizSubmissionAnswer{{ID: 1, QuizSubmissionID: 3, QuestionID: 1, Answer: "a", Correct: &correct, Points: &pts}}

	app, d := quizApp("StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.aRepo.On("ListBySubmissionID", mock.Anything, uint(3)).Return(answers, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	rows := m["quiz_submission_answers"].([]interface{})
	require.Len(t, rows, 1)
	row := rows[0].(map[string]interface{})
	assert.Nil(t, row["correct"])
	assert.Nil(t, row["points"])
	assert.Equal(t, "a", row["answer"])

	teacher, d2 := quizApp("TeacherEnrollment", 2)
	d2.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	d2.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d2.aRepo.On("ListBySubmissionID", mock.Anything, uint(3)).Return(answers, nil)
	resp = testutil.MakeRequest(teacher, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	m, err = testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	row = m["quiz_submission_answers"].([]interface{})[0].(map[string]interface{})
	assert.Equal(t, true, row["correct"])
}
