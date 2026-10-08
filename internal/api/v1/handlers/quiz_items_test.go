package handlers_test

// Handler tests for the quiz-items lane: delivery settings (G049, G055,
// G058), their enforcement while taking a quiz, per-attempt rendering
// (shuffle, formula variables, can't-go-back locks), the formula preview
// (G051) and item analysis (G054).

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func quizItemsApp(t *testing.T, role string, userID uint) (*fiber.App, *quizDeps) {
	t.Helper()
	// Access codes are sealed with secretbox.
	t.Setenv("MFA_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
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
	app.Post("/courses/:course_id/quizzes", qh.CreateQuiz)
	app.Get("/courses/:course_id/quizzes/:id", qh.GetQuiz)
	app.Put("/courses/:course_id/quizzes/:id", qh.UpdateQuiz)
	app.Get("/courses/:course_id/quizzes/:quiz_id/questions", qq.ListQuestions)
	app.Post("/courses/:course_id/quizzes/:quiz_id/formula_preview", qq.FormulaPreview)
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions", qs.StartSubmission)
	app.Put("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id", qs.AnswerQuestion)
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/read", qs.MarkQuestionRead)
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/complete", qs.CompleteSubmission)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions", qs.GetSubmissionQuestions)
	app.Get("/courses/:course_id/quizzes/:quiz_id/item_analysis", st.GetItemAnalysis)
	app.Get("/courses/:course_id/quizzes/:quiz_id/item_analysis.csv", st.ItemAnalysisCSV)
	app.Get("/courses/:course_id/quizzes/:quiz_id/student_analysis.csv", st.StudentAnalysisCSV)
	return app, d
}

func decodeMap(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	m, err := testutil.ParseJSONMap(resp)
	require.NoError(t, err)
	return m
}

// ---- G049 / G055 / G058: settings round-trip ----

func TestQuizItems_UpdateDeliverySettings(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	quiz := &models.Quiz{ID: 5, CourseID: 7, WorkflowState: "unpublished", QuizType: "survey"}
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	var saved *models.Quiz
	d.quizRepo.On("Update", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { saved = a.Get(1).(*models.Quiz) }).Return(nil)

	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{
			"shuffle_answers": true, "one_question_at_a_time": true, "cant_go_back": true,
			"access_code": " sesame ", "ip_filter": "10.0.0.0/8\n192.168.1.5", "anonymous_submissions": true,
		},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeMap(t, resp)
	assert.Equal(t, true, body["shuffle_answers"])
	assert.Equal(t, true, body["one_question_at_a_time"])
	assert.Equal(t, true, body["cant_go_back"])
	assert.Equal(t, true, body["anonymous_submissions"])
	assert.Equal(t, true, body["requires_access_code"])
	assert.Equal(t, "sesame", body["access_code"], "staff see the access code")
	assert.Equal(t, "10.0.0.0/8,192.168.1.5", body["ip_filter"])

	require.NotNil(t, saved)
	assert.NotEmpty(t, saved.AccessCodeEncrypted)
	assert.NotContains(t, string(saved.AccessCodeEncrypted), "sesame", "the code is sealed at rest")

	// can't-go-back is dropped when one-question-at-a-time is turned off.
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{"one_question_at_a_time": false},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body = decodeMap(t, resp)
	assert.Equal(t, false, body["cant_go_back"])
	assert.False(t, saved.CantGoBack)

	// Clearing the access code.
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{"access_code": ""},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, saved.AccessCodeEncrypted)
}

func TestQuizItems_InvalidIPFilterRejected(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7}, nil)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{"ip_filter": "999.1.1.1"},
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	d.quizRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestQuizItems_CreateQuizWithSettings(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	var created *models.Quiz
	d.quizRepo.On("Create", mock.Anything, mock.Anything).Run(func(a mock.Arguments) { created = a.Get(1).(*models.Quiz) }).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{"title": "Lab quiz", "shuffle_answers": true, "ip_filter": "172.16.0.0/12"},
	}))
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, created)
	assert.True(t, created.ShuffleAnswers)
	assert.Equal(t, "172.16.0.0/12", created.IPFilter)
}

// Students learn a code is required but never see it or the IP filter.
func sealedQuiz(t *testing.T, code string) *models.Quiz {
	t.Helper()
	q := publishedQuiz(5, 7)
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.quizRepo.On("Update", mock.Anything, mock.Anything).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5", testutil.JSONBody(map[string]interface{}{
		"quiz": map[string]interface{}{"access_code": code},
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return q
}

func TestQuizItems_StudentQuizJSONHidesSecrets(t *testing.T) {
	q := sealedQuiz(t, "sesame")
	q.IPFilter = "10.0.0.0/8"
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	assert.NotContains(t, string(raw), "sesame")
	assert.NotContains(t, string(raw), "10.0.0.0")
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body))
	assert.Equal(t, true, body["requires_access_code"])
	assert.Equal(t, true, body["has_ip_filter"])
	_, hasCode := body["access_code"]
	assert.False(t, hasCode)

	// Outside an attempt the question list is withheld on a gated quiz.
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/questions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Empty(t, arr)
	d.qRepo.AssertNotCalled(t, "ListByQuizID", mock.Anything, mock.Anything, mock.Anything)
}

// ---- G055: access code + IP filter enforcement ----

func TestQuizItems_AccessCodeRequiredToStart(t *testing.T) {
	q := sealedQuiz(t, "sesame")
	for _, tc := range []struct {
		name string
		body interface{}
		want int
	}{
		{"missing", map[string]string{}, http.StatusForbidden},
		{"wrong", map[string]string{"access_code": "open"}, http.StatusForbidden},
		{"right", map[string]string{"access_code": "sesame"}, http.StatusCreated},
	} {
		app, d := quizItemsApp(t, "StudentEnrollment", 10)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(0)).Return(q, nil)
		d.sRepo.On("FindByQuizAndUser", mock.Anything, uint(5), uint(10)).Return(nil, nil)
		d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(), nil)
		d.sRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

		resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions", testutil.JSONBody(tc.body))
		assert.Equal(t, tc.want, resp.StatusCode, tc.name)
		if tc.want == http.StatusForbidden {
			d.sRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		}
	}

	// Staff preview without the code.
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(0)).Return(q, nil)
	d.sRepo.On("FindByQuizAndUser", mock.Anything, uint(5), uint(2)).Return(nil, nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(), nil)
	d.sRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions", nil)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestQuizItems_IPFilterEnforcedWhileTaking(t *testing.T) {
	// fiber's test transport reports the peer as 0.0.0.0.
	denied := publishedQuiz(5, 7)
	denied.IPFilter = "10.0.0.0/8"
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(denied, nil)
	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/courses/7/quizzes/5/submissions"},
		{http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/1"},
		{http.MethodPost, "/courses/7/quizzes/5/submissions/3/complete"},
	} {
		resp := testutil.MakeRequest(app, req.method, req.path, testutil.JSONBody(map[string]string{"answer": "a"}))
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, req.path)
	}
	d.sRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)

	allowed := publishedQuiz(5, 7)
	allowed.IPFilter = "0.0.0.0/32"
	app, d = quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(allowed, nil)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(0)).Return(allowed, nil)
	d.sRepo.On("FindByQuizAndUser", mock.Anything, uint(5), uint(10)).Return(nil, nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(), nil)
	d.sRepo.On("Create", mock.Anything, mock.Anything).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions", nil)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

// A client talking to the app directly cannot claim an allowed address via
// X-Forwarded-For (the header is only honored from a private/loopback peer).
func TestQuizItems_IPFilterIgnoresSpoofedForwardedFor(t *testing.T) {
	q := publishedQuiz(5, 7)
	q.IPFilter = "10.0.0.0/8"
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	req := httptest.NewRequest(http.MethodPost, "/courses/7/quizzes/5/submissions", nil)
	req.Header.Set("X-Forwarded-For", "10.1.1.1")
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// ---- G049: shuffle answers + can't go back ----

const fiveChoices = `[{"id":"a","text":"A","weight":100},{"id":"b","text":"B","weight":0},{"id":"c","text":"C","weight":0},{"id":"d","text":"D","weight":0},{"id":"e","text":"E","weight":0}]`

func choiceOrder(t *testing.T, answers interface{}) string {
	t.Helper()
	var list []struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal([]byte(answers.(string)), &list))
	ids := make([]string, len(list))
	for i, o := range list {
		ids[i] = o.ID
	}
	return strings.Join(ids, "")
}

func TestQuizItems_ShuffleAnswersPerAttempt(t *testing.T) {
	q := publishedQuiz(5, 7)
	q.ShuffleAnswers = true
	orders := map[string]bool{}
	for sub := uint(1); sub <= 12; sub++ {
		app, d := quizItemsApp(t, "StudentEnrollment", 10)
		d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
		d.sRepo.On("FindByID", mock.Anything, sub).Return(&models.QuizSubmission{ID: sub, QuizID: 5, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[1]"}, nil)
		d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: fiveChoices}, nil)
		path := "/courses/7/quizzes/5/submissions/" + itoa(sub) + "/questions"
		resp := testutil.MakeRequest(app, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		arr, _ := testutil.ParseJSONArray(resp)
		first := choiceOrder(t, arr[0]["answers"])
		assert.NotContains(t, arr[0]["answers"].(string), "weight")
		// Stable across reloads of the same attempt.
		resp = testutil.MakeRequest(app, http.MethodGet, path, nil)
		arr, _ = testutil.ParseJSONArray(resp)
		assert.Equal(t, first, choiceOrder(t, arr[0]["answers"]))
		orders[first] = true
	}
	assert.Greater(t, len(orders), 3, "different attempts should see different orders")

	// Teachers see the authored order.
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[1]"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: fiveChoices}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions", nil)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Equal(t, "abcde", choiceOrder(t, arr[0]["answers"]))
}

func itoa(n uint) string { return strconv.FormatUint(uint64(n), 10) }

func TestQuizItems_CantGoBackLocksQuestions(t *testing.T) {
	q := publishedQuiz(5, 7)
	q.OneQuestionAtATime, q.CantGoBack = true, true
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[1,2]"}
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(0)).Return(q, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.sRepo.On("Update", mock.Anything, sub).Return(nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(2)).Return(&models.QuizQuestion{ID: 2, QuizID: 5, QuestionType: "short_answer", Answers: `[]`}, nil)
	d.aRepo.On("FindBySubmissionAndQuestion", mock.Anything, uint(3), uint(2)).Return(nil, nil)
	d.aRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	// Moving past question 1.
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/1/read", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "[1]", sub.ReadQuestions)

	// Question 1 can no longer be answered; question 2 still can.
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/1", testutil.JSONBody(map[string]string{"answer": "a"}))
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	resp = testutil.MakeRequest(app, http.MethodPut, "/courses/7/quizzes/5/submissions/3/questions/2", testutil.JSONBody(map[string]string{"answer": "Paris"}))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// The attempt's question list flags the lock.
	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions", nil)
	arr, _ := testutil.ParseJSONArray(resp)
	require.Len(t, arr, 2)
	assert.Equal(t, true, arr[0]["locked"])
	assert.Nil(t, arr[1]["locked"])

	// Another student cannot lock this attempt's questions.
	other, d2 := quizItemsApp(t, "StudentEnrollment", 11)
	d2.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d2.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	resp = testutil.MakeRequest(other, http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/2/read", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestQuizItems_MarkReadIsNoopWithoutCantGoBack(t *testing.T) {
	q := publishedQuiz(5, 7)
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[1]"}
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), mock.Anything).Return(q, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "multiple_choice", Answers: mcKey}, nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/1/read", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Empty(t, sub.ReadQuestions)
	d.sRepo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// ---- G051: per-attempt formula variables ----

const formulaKey = `[{"id":"f","weight":100,"formula":"x*2","variables":[{"name":"x","min":1,"max":100,"precision":0}],"tolerance":"0"}]`

func TestQuizItems_FormulaQuestionPersonalized(t *testing.T) {
	q := publishedQuiz(5, 7)
	question := &models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "formula", QuestionText: "<p>Double [x].</p>", Answers: formulaKey}
	app, d := quizItemsApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(q, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken", SelectedQuestions: "[1]"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(question, nil)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	require.Len(t, arr, 1)
	text := arr[0]["question_text"].(string)
	assert.NotContains(t, text, "[x]")
	assert.Equal(t, "[]", arr[0]["answers"], "the formula is the key: redacted while taking")
	_, leaked := arr[0]["formula_answer"]
	assert.False(t, leaked)
	vars := arr[0]["formula_variables"].([]interface{})
	require.Len(t, vars, 1)
	x := vars[0].(map[string]interface{})["value"].(float64)
	assert.Contains(t, text, service.FormatFormulaNumber(x))
}

func TestQuizItems_FormulaPreview(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 7}, nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/formula_preview", testutil.JSONBody(map[string]interface{}{
		"answers": formulaKey, "count": 4,
	}))
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeMap(t, resp)
	samples := body["samples"].([]interface{})
	require.Len(t, samples, 4)
	for _, s := range samples {
		m := s.(map[string]interface{})
		x := m["variables"].([]interface{})[0].(map[string]interface{})["value"].(float64)
		assert.Equal(t, x*2, m["expected"].(float64))
	}

	resp = testutil.MakeRequest(app, http.MethodPost, "/courses/7/quizzes/5/formula_preview", testutil.JSONBody(map[string]interface{}{
		"answers": `[{"formula":"x +","variables":[{"name":"x","min":1,"max":2}]}]`,
	}))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---- G054: item analysis ----

func analysisFixture(d *quizDeps, quiz *models.Quiz) {
	pts := 1.0
	two := 2.0
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	d.qRepo.On("ListByQuizID", mock.Anything, uint(5), mock.Anything).Return(questionPage(
		models.QuizQuestion{ID: 1, QuizID: 5, Position: 1, QuestionType: "multiple_choice", QuestionText: "<p>2+2?</p>", PointsPossible: &pts, Answers: mcKey},
		models.QuizQuestion{ID: 2, QuizID: 5, Position: 2, QuestionType: "short_answer", QuestionText: "Capital of France?", PointsPossible: &two, Answers: `[{"id":"s","text":"Paris","weight":100}]`},
	), nil)
	score := func(v float64) *float64 { return &v }
	now := time.Now()
	d.sRepo.On("ListCompletedByQuizID", mock.Anything, uint(5)).Return([]models.QuizSubmission{
		{ID: 11, QuizID: 5, UserID: 101, Attempt: 1, WorkflowState: "complete", Score: score(0), FinishedAt: &now},
		{ID: 12, QuizID: 5, UserID: 101, Attempt: 2, WorkflowState: "complete", Score: score(3), FinishedAt: &now}, // latest wins
		{ID: 13, QuizID: 5, UserID: 102, Attempt: 1, WorkflowState: "complete", Score: score(1), FinishedAt: &now},
		{ID: 14, QuizID: 5, UserID: 103, Attempt: 1, WorkflowState: "complete", Score: score(0), FinishedAt: &now},
	}, nil)
	yes, no := true, false
	auto := "auto"
	d.aRepo.On("ListBySubmissionIDs", mock.Anything, []uint{12, 13, 14}).Return([]models.QuizSubmissionAnswer{
		{QuizSubmissionID: 12, QuestionID: 1, Answer: "a", Correct: &yes, Points: score(1), GradedVia: &auto},
		{QuizSubmissionID: 12, QuestionID: 2, Answer: "Paris", Correct: &yes, Points: score(2), GradedVia: &auto},
		{QuizSubmissionID: 13, QuestionID: 1, Answer: "a", Correct: &yes, Points: score(1), GradedVia: &auto},
		{QuizSubmissionID: 13, QuestionID: 2, Answer: "=HYPERLINK(1)", Correct: &no, Points: score(0), GradedVia: &auto},
		{QuizSubmissionID: 14, QuestionID: 1, Answer: "b", Correct: &no, Points: score(0), GradedVia: &auto},
	}, nil)
}

func TestQuizItems_ItemAnalysisReport(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	quiz := &models.Quiz{ID: 5, CourseID: 7, Title: "Unit 1"}
	analysisFixture(d, quiz)

	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/item_analysis", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var report service.ItemAnalysisReport
	require.NoError(t, testutil.ParseJSONResponse(resp, &report))
	assert.Equal(t, 3, report.Summary.Respondents, "one (latest) attempt per student")
	require.Len(t, report.Items, 2)
	mc := report.Items[0]
	assert.Equal(t, 3, mc.Respondents)
	assert.Equal(t, 2, mc.Correct)
	assert.Equal(t, 1, mc.Incorrect)
	require.NotNil(t, mc.Difficulty)
	assert.InDelta(t, 0.67, *mc.Difficulty, 0.01)
	require.NotNil(t, mc.Discrimination)
	assert.InDelta(t, 1.0, *mc.Discrimination, 0.001, "top scorer right, bottom scorer wrong")
	require.NotNil(t, mc.PointBiserial)
	assert.Greater(t, *mc.PointBiserial, 0.0)
	require.Len(t, mc.Options, 2)
	assert.Equal(t, 2, mc.Options[0].Count)
	assert.True(t, mc.Options[0].Correct)
	sa := report.Items[1]
	assert.Equal(t, 1, sa.Unanswered)
	assert.NotEmpty(t, sa.Responses)
	assert.Equal(t, "Capital of France?", sa.QuestionText)
	assert.Equal(t, "2+2?", mc.QuestionText, "question text is plain text")
}

func TestQuizItems_ItemAnalysisCSVs(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	analysisFixture(d, &models.Quiz{ID: 5, CourseID: 7, Title: "Unit 1"})
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/item_analysis.csv", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/csv")
	assert.Contains(t, resp.Header.Get("Content-Disposition"), "attachment")
	raw, _ := io.ReadAll(resp.Body)
	assert.Contains(t, string(raw), "Discrimination Index")
	assert.Contains(t, string(raw), "Cronbach alpha")

	resp = testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/student_analysis.csv", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ = io.ReadAll(resp.Body)
	csvText := string(raw)
	assert.Contains(t, csvText, "'=HYPERLINK(1)", "formula injection is neutralized")
	assert.Contains(t, csvText, "101")
}

func TestQuizItems_StudentAnalysisAnonymousSurvey(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	analysisFixture(d, &models.Quiz{ID: 5, CourseID: 7, Title: "Feedback", QuizType: "survey", AnonymousSubmissions: true})
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/student_analysis.csv", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 4)
	for _, line := range lines[1:] {
		assert.True(t, strings.HasPrefix(line, "Anonymous "), line)
		assert.NotContains(t, line, ",101,")
	}
}

// G058: on an anonymous survey, staff cannot open one respondent's answers;
// the respondent still can.
func TestQuizItems_AnonymousSurveyAnswersHiddenFromStaff(t *testing.T) {
	quiz := publishedQuiz(5, 7)
	quiz.QuizType, quiz.AnonymousSubmissions = "survey", true
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "complete", Attempt: 1}
	answers := []models.QuizSubmissionAnswer{{ID: 1, QuizSubmissionID: 3, QuestionID: 1, Answer: "honest feedback"}}

	teacher, d := quizItemsApp(t, "TeacherEnrollment", 2)
	teacher.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers", handlers.NewQuizSubmissionHandler(service.NewQuizService(d.quizRepo, d.qRepo, d.sRepo, d.aRepo), nil, nil).GetSubmissionAnswers)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.aRepo.On("ListBySubmissionID", mock.Anything, uint(3)).Return(answers, nil)
	resp := testutil.MakeRequest(teacher, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	student, d2 := quizItemsApp(t, "StudentEnrollment", 10)
	student.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/answers", handlers.NewQuizSubmissionHandler(service.NewQuizService(d2.quizRepo, d2.qRepo, d2.sRepo, d2.aRepo), nil, nil).GetSubmissionAnswers)
	d2.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(quiz, nil)
	d2.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d2.aRepo.On("ListBySubmissionID", mock.Anything, uint(3)).Return(answers, nil)
	resp = testutil.MakeRequest(student, http.MethodGet, "/courses/7/quizzes/5/submissions/3/answers", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestQuizItems_ItemAnalysisScopedToCourse(t *testing.T) {
	app, d := quizItemsApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(&models.Quiz{ID: 5, CourseID: 99}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/item_analysis", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "a quiz from another course is not found")
}
