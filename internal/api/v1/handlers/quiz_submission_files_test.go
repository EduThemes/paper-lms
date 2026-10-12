package handlers_test

// B127: file-upload quiz questions store a real file and teachers can
// retrieve it; nobody else can.

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/storage"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

type fileQuizDeps struct {
	quizDeps
	attRepo *mocks.MockAttachmentRepository
}

func fileQuizApp(t *testing.T, role string, userID uint) (*fiber.App, *fileQuizDeps) {
	t.Helper()
	d := &fileQuizDeps{
		quizDeps: quizDeps{
			quizRepo: new(mocks.MockQuizRepository),
			qRepo:    new(mocks.MockQuizQuestionRepository),
			sRepo:    new(mocks.MockQuizSubmissionRepository),
			aRepo:    new(mocks.MockQuizSubmissionAnswerRepository),
		},
		attRepo: new(mocks.MockAttachmentRepository),
	}
	svc := service.NewQuizService(d.quizRepo, d.qRepo, d.sRepo, d.aRepo)
	fs := service.NewFileServiceWithBackend(new(mocks.MockFolderRepository), d.attRepo, storage.NewLocalBackend(t.TempDir()))
	h := handlers.NewQuizSubmissionHandler(svc, nil, nil)
	h.SetFileService(fs)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("account_id", uint(1))
		if role != "" {
			c.Locals("enrollment_type", role)
		}
		return c.Next()
	})
	app.Post("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/file", h.UploadAnswerFile)
	app.Get("/courses/:course_id/quizzes/:quiz_id/submissions/:submission_id/questions/:question_id/file", h.DownloadAnswerFile)
	return app, d
}

func multipartFile(t *testing.T, name, content string) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, _ = fw.Write([]byte(content))
	require.NoError(t, w.Close())
	return &buf, w.FormDataContentType()
}

func TestQuizFileUpload_StoresAttachmentAndAnswer(t *testing.T) {
	app, d := fileQuizApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken"}
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "file_upload"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(2)).Return(&models.QuizQuestion{ID: 2, QuizID: 5, QuestionType: "essay"}, nil)
	var saved *models.Attachment
	d.attRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Attachment")).Run(func(a mock.Arguments) {
		saved = a.Get(1).(*models.Attachment)
		saved.ID = 77
	}).Return(nil)
	d.aRepo.On("FindBySubmissionAndQuestion", mock.Anything, uint(3), uint(1)).Return(nil, errors.New("none"))
	var stored string
	d.aRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.QuizSubmissionAnswer")).Run(func(a mock.Arguments) {
		stored = a.Get(1).(*models.QuizSubmissionAnswer).Answer
	}).Return(nil)

	body, ct := multipartFile(t, "essay.txt", "hello world")
	req := httptest.NewRequest(http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/1/file", body)
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	require.NotNil(t, saved)
	assert.Equal(t, "User", saved.ContextType)
	assert.Equal(t, uint(10), saved.ContextID)
	assert.Contains(t, stored, `"attachment_id":77`)
	assert.Contains(t, stored, `"filename":"essay.txt"`)

	// A non-file_upload question refuses uploads.
	body, ct = multipartFile(t, "x.txt", "x")
	req = httptest.NewRequest(http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/2/file", body)
	req.Header.Set("Content-Type", ct)
	resp, err = app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestQuizFileUpload_OtherStudentCannotUploadIntoAttempt(t *testing.T) {
	app, d := fileQuizApp(t, "StudentEnrollment", 11)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken"}, nil)

	body, ct := multipartFile(t, "x.txt", "x")
	req := httptest.NewRequest(http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/1/file", body)
	req.Header.Set("Content-Type", ct)
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d.attRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestQuizFileDownload_TeacherYesOtherStudentNo(t *testing.T) {
	sub := &models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "pending_review"}

	// Teacher: the recorded attachment is streamed.
	app, d := fileQuizApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d.aRepo.On("FindBySubmissionAndQuestion", mock.Anything, uint(3), uint(1)).
		Return(&models.QuizSubmissionAnswer{ID: 9, QuestionID: 1, Answer: `{"attachment_id":77,"filename":"essay.txt"}`}, nil)
	d.attRepo.On("FindByID", mock.Anything, uint(77), uint(1)).
		Return(&models.Attachment{ID: 77, ContextType: "User", ContextID: 10, Filename: "essay.txt", StoragePath: "missing/essay.txt"}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions/1/file", nil)
	// The fake path is not in storage, so the stream fails with 500 — but
	// authorization passed (not 404).
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode)

	// Attachment that is not the attempt owner's upload → 404.
	app2, d2 := fileQuizApp(t, "TeacherEnrollment", 2)
	d2.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d2.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	d2.aRepo.On("FindBySubmissionAndQuestion", mock.Anything, uint(3), uint(1)).
		Return(&models.QuizSubmissionAnswer{ID: 9, QuestionID: 1, Answer: `{"attachment_id":88}`}, nil)
	d2.attRepo.On("FindByID", mock.Anything, uint(88), uint(1)).
		Return(&models.Attachment{ID: 88, ContextType: "User", ContextID: 999}, nil)
	resp = testutil.MakeRequest(app2, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions/1/file", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Another student → 404 before anything is loaded.
	app3, d3 := fileQuizApp(t, "StudentEnrollment", 11)
	d3.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(publishedQuiz(5, 7), nil)
	d3.sRepo.On("FindByID", mock.Anything, uint(3)).Return(sub, nil)
	resp = testutil.MakeRequest(app3, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions/1/file", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d3.aRepo.AssertNotCalled(t, "FindBySubmissionAndQuestion", mock.Anything, mock.Anything, mock.Anything)
}

// Quiz-items lane: a file upload is an answer, so the IP filter and the
// can't-go-back lock apply before anything is stored.
func TestQuizFileUpload_DeliveryRestrictionsApply(t *testing.T) {
	upload := func(app *fiber.App) int {
		body, ct := multipartFile(t, "x.txt", "x")
		req := httptest.NewRequest(http.MethodPost, "/courses/7/quizzes/5/submissions/3/questions/1/file", body)
		req.Header.Set("Content-Type", ct)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		return resp.StatusCode
	}

	// IP filter (fiber's test transport reports the peer as 0.0.0.0).
	filtered := publishedQuiz(5, 7)
	filtered.IPFilter = "10.0.0.0/8"
	app, d := fileQuizApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(filtered, nil)
	assert.Equal(t, http.StatusForbidden, upload(app))
	d.sRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
	d.attRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)

	// Can't go back: question 1 was already moved past.
	locked := publishedQuiz(5, 7)
	locked.OneQuestionAtATime, locked.CantGoBack = true, true
	app, d = fileQuizApp(t, "StudentEnrollment", 10)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), mock.Anything).Return(locked, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "untaken", ReadQuestions: "[1]"}, nil)
	d.qRepo.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizQuestion{ID: 1, QuizID: 5, QuestionType: "file_upload"}, nil)
	assert.Equal(t, http.StatusForbidden, upload(app))
	d.attRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// On an anonymous survey staff cannot open one respondent's upload.
func TestQuizFileDownload_AnonymousSurveyHiddenFromStaff(t *testing.T) {
	survey := publishedQuiz(5, 7)
	survey.QuizType, survey.AnonymousSubmissions = "survey", true
	app, d := fileQuizApp(t, "TeacherEnrollment", 2)
	d.quizRepo.On("FindByID", mock.Anything, uint(5), uint(1)).Return(survey, nil)
	d.sRepo.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizSubmission{ID: 3, QuizID: 5, UserID: 10, WorkflowState: "complete"}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/7/quizzes/5/submissions/3/questions/1/file", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	d.aRepo.AssertNotCalled(t, "FindBySubmissionAndQuestion", mock.Anything, mock.Anything, mock.Anything)
}
