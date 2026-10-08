package handlers_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/EduThemes/paper-lms/internal/api/v1/handlers"
	"github.com/EduThemes/paper-lms/internal/domain/models"
	"github.com/EduThemes/paper-lms/internal/service"
	"github.com/EduThemes/paper-lms/internal/testutil"
	"github.com/EduThemes/paper-lms/internal/testutil/mocks"
)

func setupStimulusHandler() (*fiber.App, *mocks.MockQuizStimulusRepository, *mocks.MockQuizQuestionRepository) {
	sr := new(mocks.MockQuizStimulusRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	svc := service.NewQuizStimulusService(sr, qr)
	h := handlers.NewQuizStimulusHandler(svc)

	app := testutil.SetupTestApp()

	app.Post("/courses/:course_id/quiz_stimuli", h.CreateStimulus)
	app.Get("/courses/:course_id/quiz_stimuli/:stimulus_id", h.GetStimulus)
	app.Put("/courses/:course_id/quiz_stimuli/:stimulus_id", h.UpdateStimulus)
	app.Delete("/courses/:course_id/quiz_stimuli/:stimulus_id", h.DeleteStimulus)

	return app, sr, qr
}

func TestCreateStimulus_Success(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("Create", mock.Anything, mock.MatchedBy(func(s *models.QuizStimulus) bool {
		return s.CourseID == 5 && s.Title == "Passage"
	})).Return(nil)

	body := testutil.JSONBody(map[string]interface{}{
		"stimulus": map[string]string{"title": "Passage", "content": "{}"},
	})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli", body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestCreateStimulus_MissingTitle(t *testing.T) {
	app, _, _ := setupStimulusHandler()
	body := testutil.JSONBody(map[string]interface{}{"stimulus": map[string]string{}})
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetStimulus_CrossCourse(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/6/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestGetStimulus_NotFound(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(nil, errors.New("nope"))
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/5/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUpdateStimulus_PartialPatch(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5, Title: "Old"}, nil).Twice()
	sr.On("Update", mock.Anything, mock.MatchedBy(func(s *models.QuizStimulus) bool {
		return s.Title == "New"
	})).Return(nil)
	body := testutil.JSONBody(map[string]interface{}{"stimulus": map[string]string{"title": "New"}})
	resp := testutil.MakeRequest(app, http.MethodPut, "/courses/5/quiz_stimuli/1", body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDeleteStimulus_Success(t *testing.T) {
	app, sr, _ := setupStimulusHandler()
	sr.On("FindByID", mock.Anything, uint(1)).Return(&models.QuizStimulus{ID: 1, CourseID: 5}, nil)
	sr.On("Delete", mock.Anything, uint(1)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/5/quiz_stimuli/1", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Link / unlink / list are course-scoped: the stimulus must be in
// :course_id and the question's quiz in the same course + tenant.
func setupScopedStimulusHandler(role string) (*fiber.App, *mocks.MockQuizStimulusRepository, *mocks.MockQuizQuestionRepository, *mocks.MockQuizRepository) {
	sr := new(mocks.MockQuizStimulusRepository)
	qr := new(mocks.MockQuizQuestionRepository)
	quizRepo := new(mocks.MockQuizRepository)
	svc := service.NewQuizStimulusService(sr, qr)
	svc.SetQuizRepo(quizRepo)
	h := handlers.NewQuizStimulusHandler(svc)

	app := testutil.SetupTestApp()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", uint(2))
		c.Locals("account_id", uint(1))
		c.Locals("enrollment_type", role)
		return c.Next()
	})
	app.Post("/courses/:course_id/quiz_stimuli/:stimulus_id/questions/:question_id", h.LinkQuestion)
	app.Delete("/courses/:course_id/quiz_stimuli/:stimulus_id/questions/:question_id", h.UnlinkQuestion)
	app.Get("/courses/:course_id/quiz_stimuli/:stimulus_id/questions", h.ListQuestions)
	return app, sr, qr, quizRepo
}

func TestLinkQuestion_Success(t *testing.T) {
	app, sr, qr, quizRepo := setupScopedStimulusHandler("TeacherEnrollment")
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3, CourseID: 5}, nil)
	qr.On("FindByID", mock.Anything, uint(7)).Return(&models.QuizQuestion{ID: 7, QuizID: 9}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 5}, nil)
	sr.On("SetQuestionStimulus", mock.Anything, uint(7), mock.MatchedBy(func(sid *uint) bool {
		return sid != nil && *sid == 3
	})).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli/3/questions/7", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLinkQuestion_QuestionFromOtherCourse_404(t *testing.T) {
	app, sr, qr, quizRepo := setupScopedStimulusHandler("TeacherEnrollment")
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3, CourseID: 5}, nil)
	qr.On("FindByID", mock.Anything, uint(7)).Return(&models.QuizQuestion{ID: 7, QuizID: 9}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(nil, errors.New("other tenant"))
	resp := testutil.MakeRequest(app, http.MethodPost, "/courses/5/quiz_stimuli/3/questions/7", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	sr.AssertNotCalled(t, "SetQuestionStimulus", mock.Anything, mock.Anything, mock.Anything)
}

func TestUnlinkQuestion_Success(t *testing.T) {
	app, sr, qr, quizRepo := setupScopedStimulusHandler("TeacherEnrollment")
	sid := uint(3)
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3, CourseID: 5}, nil)
	qr.On("FindByID", mock.Anything, uint(7)).Return(&models.QuizQuestion{ID: 7, QuizID: 9, StimulusID: &sid}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 5}, nil)
	sr.On("SetQuestionStimulus", mock.Anything, uint(7), (*uint)(nil)).Return(nil)
	resp := testutil.MakeRequest(app, http.MethodDelete, "/courses/5/quiz_stimuli/3/questions/7", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestListQuestions_Success(t *testing.T) {
	app, sr, _, quizRepo := setupScopedStimulusHandler("TeacherEnrollment")
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3, CourseID: 5}, nil)
	sr.On("ListQuestionsForStimulus", mock.Anything, uint(3)).Return([]models.QuizQuestion{{ID: 1, QuizID: 9}, {ID: 2, QuizID: 9}, {ID: 3, QuizID: 10}}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(9), uint(1)).Return(&models.Quiz{ID: 9, CourseID: 5}, nil)
	quizRepo.On("FindByID", mock.Anything, uint(10), uint(1)).Return(&models.Quiz{ID: 10, CourseID: 99}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/5/quiz_stimuli/3/questions", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	arr, _ := testutil.ParseJSONArray(resp)
	assert.Len(t, arr, 2, "questions from another course are not listed")
}

// The listing carries answer keys: a stimulus from another course is
// invisible and students are refused outright.
func TestListQuestions_CrossCourseAndStudent_404(t *testing.T) {
	app, sr, _, _ := setupScopedStimulusHandler("TeacherEnrollment")
	sr.On("FindByID", mock.Anything, uint(3)).Return(&models.QuizStimulus{ID: 3, CourseID: 99}, nil)
	resp := testutil.MakeRequest(app, http.MethodGet, "/courses/5/quiz_stimuli/3/questions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	sr.AssertNotCalled(t, "ListQuestionsForStimulus", mock.Anything, mock.Anything)

	app2, sr2, _, _ := setupScopedStimulusHandler("StudentEnrollment")
	resp = testutil.MakeRequest(app2, http.MethodGet, "/courses/5/quiz_stimuli/3/questions", nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	sr2.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
}
